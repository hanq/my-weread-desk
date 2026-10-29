package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/device"
	"my-weread-desk/internal/weread/httpx"
	"my-weread-desk/internal/weread/protocol"
)

// The login handshake is the official E-Ink client's WeChat QR flow, not the
// browser one:
//
//	wxticket (sign a WeChat SDK request) -> open.weixin.qq.com/connect/sdk/qrconnect
//	-> long.open.weixin.qq.com/connect/l/qrconnect (scan / confirm)
//	-> i.weread.qq.com/login with the one-time wx code
//
// Only the last step mints credentials the i.weread.qq.com API accepts. The
// browser flow (weread.qq.com/api/auth/getLoginInfo) returns a web-scoped
// session instead: its access token is rejected by the API with -2012 and its
// `web@...` refresh token by /login with -2013, so it cannot be used here.
const (
	einkBase  = "https://i.weread.qq.com"
	webBase   = "https://weread.qq.com"
	wxConnect = "https://open.weixin.qq.com/connect/sdk/qrconnect"
	wxLong    = "https://long.open.weixin.qq.com/connect/l/qrconnect"

	wxAppID = "wxab9b71ad2b90ff34"
	wxScope = "snsapi_userinfo,snsapi_timeline,snsapi_friend"

	// The WeChat long poll holds the connection open for about 60 seconds.
	pollTimeout  = 70 * time.Second
	loginTimeout = 5 * time.Minute
)

// endpoints groups the handshake URLs so tests can point them at a stub server.
type endpoints struct {
	ticket string
	qr     string
	poll   string
	login  string
	user   string
	apiKey string
}

func defaultEndpoints() endpoints {
	return endpoints{
		ticket: einkBase + "/wxticket?nonceStr=weread",
		qr:     wxConnect,
		poll:   wxLong,
		login:  einkBase + "/login",
		user:   webBase + "/api/userInfo",
		apiKey: webBase + "/api/skills/apikeyGet?only_show=1",
	}
}

// Login drives the WeChat QR login handshake.
type Login struct {
	http  *httpx.Client
	store *Store
	ep    endpoints

	startedAt time.Time

	lastMu sync.Mutex
	last   string
}

// NewLogin creates a login helper bound to a credential store.
func NewLogin(store *Store) *Login {
	return newLogin(store, defaultEndpoints())
}

func newLogin(store *Store, ep endpoints) *Login {
	c := httpx.New(true)
	c.UserAgent = device.UserAgent
	for k, v := range device.Headers() {
		c.SetHeader(k, v)
	}
	return &Login{http: c, store: store, ep: ep}
}

// PollResult is one poll outcome.
//
// `Pending` means "keep polling the same UID". `Expired` means "stop polling
// and let the user ask for a new code": the client must never mint a new QR
// code on its own, because that turns a slow scan into a request storm.
type PollResult struct {
	Pending bool
	NeedOTP bool
	Done    bool
	Expired bool
	Message string
	Account *weread.Account
}

// Start asks for a fresh WeChat QR challenge.
func (l *Login) Start(ctx context.Context) (*weread.QRCode, error) {
	l.startedAt = time.Now()
	l.setLast("")

	var ticket struct {
		Signature string            `json:"signature"`
		TimeStamp weread.FlexString `json:"timeStamp"`
		ExpiresIn weread.FlexInt    `json:"expires_in"`
	}
	if err := l.http.JSON(ctx, httpx.Request{
		URL:    l.ep.ticket,
		Method: "GET",
	}, &ticket); err != nil {
		return nil, err
	}
	if ticket.Signature == "" || ticket.TimeStamp.String() == "" {
		return nil, fmt.Errorf("%w: wxticket 未返回签名", weread.ErrProtocolChanged)
	}

	q, _ := url.Parse(l.ep.qr)
	params := q.Query()
	params.Set("appid", wxAppID)
	params.Set("noncestr", "weread")
	params.Set("timestamp", ticket.TimeStamp.String())
	params.Set("scope", wxScope)
	params.Set("signature", ticket.Signature)
	q.RawQuery = params.Encode()

	var challenge struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UUID    string `json:"uuid"`
		QRCode  struct {
			Base64 string `json:"qrcodebase64"`
		} `json:"qrcode"`
	}
	if err := l.http.JSON(ctx, httpx.Request{
		URL:     q.String(),
		Method:  "GET",
		Headers: map[string]string{"User-Agent": protocol.DefaultUserAgent},
	}, &challenge); err != nil {
		return nil, err
	}
	if challenge.ErrCode != 0 || challenge.UUID == "" {
		return nil, fmt.Errorf("微信未返回登录二维码：%d %s", challenge.ErrCode, challenge.ErrMsg)
	}

	// WeChat renders the QR image itself; encoding the confirm URL is only a
	// fallback for a response that omits it.
	image := ""
	if b64 := strings.TrimSpace(challenge.QRCode.Base64); b64 != "" {
		image = "data:image/jpeg;base64," + b64
	} else {
		payload := "https://open.weixin.qq.com/connect/confirm?uuid=" + url.QueryEscape(challenge.UUID)
		png, err := qrcode.Encode(payload, qrcode.Medium, 512)
		if err != nil {
			return nil, err
		}
		image = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}

	expires := time.Now().Add(loginTimeout)
	if n := ticket.ExpiresIn.Int64(); n > 0 {
		expires = time.Now().Add(time.Duration(n) * time.Second)
	}

	return &weread.QRCode{
		UID:     challenge.UUID,
		Payload: q.String(),
		Image:   image,
		Expires: expires.Unix(),
	}, nil
}

// Poll checks the login state once. The second argument is unused and kept for
// the OTP-era binding; the WeChat flow confirms on the phone instead.
//
// Protocol notes, verified against the live service:
//   - `wx_errcode` 408 means "nobody scanned yet" and is the normal outcome of
//     a long poll; it maps to Pending, never to Expired.
//   - 404 means scanned but not confirmed, 405 means confirmed with a one-time
//     `wx_code`, 402/403 mean the code expired or was declined.
//   - The request is never retried: a retry would double the request rate for
//     no benefit and is exactly what triggers risk control.
func (l *Login) Poll(ctx context.Context, uid, _ string) (*PollResult, error) {
	if uid == "" {
		return &PollResult{Expired: true, Message: "登录会话无效，请重新获取二维码"}, nil
	}
	if !l.startedAt.IsZero() && time.Since(l.startedAt) > loginTimeout {
		return &PollResult{Expired: true, Message: "二维码已过期，请重新获取"}, nil
	}

	u := l.ep.poll + "?f=json&uuid=" + url.QueryEscape(uid)
	if last := l.getLast(); last != "" {
		u += "&last=" + url.QueryEscape(last)
	}

	resp, err := l.http.Do(ctx, httpx.Request{
		URL:     u,
		Method:  "GET",
		Timeout: pollTimeout,
		NoRetry: true,
		Headers: map[string]string{
			"User-Agent": protocol.DefaultUserAgent,
			"Accept":     "application/json, text/plain, */*",
		},
	})
	if err != nil {
		// A cancelled or timed-out long poll carries no information: the
		// session is still valid, so keep polling instead of erroring out.
		if ctx.Err() != nil {
			return &PollResult{Expired: true, Message: "登录已取消"}, nil
		}
		return &PollResult{Pending: true}, nil
	}
	if resp.Status != 200 {
		return &PollResult{Pending: true}, nil
	}
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return &PollResult{Pending: true}, nil
	}

	var body struct {
		ErrCode weread.FlexInt `json:"wx_errcode"`
		Code    string         `json:"wx_code"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("%w: 微信扫码轮询返回了非 JSON 内容", weread.ErrProtocolChanged)
	}
	code := body.ErrCode.Int64()
	l.setLast(strconv.FormatInt(code, 10))

	switch code {
	case 405:
		code := strings.TrimSpace(body.Code)
		if code == "" {
			return &PollResult{Expired: true, Message: "微信未返回登录凭证，请重新获取二维码"}, nil
		}
		creds, err := l.exchange(ctx, code)
		if err != nil {
			return nil, err
		}
		return &PollResult{Done: true, Message: "登录成功", Account: &weread.Account{
			VID: creds.VID, Name: creds.Name, Avatar: creds.Avatar, HasKey: creds.APIKey != "",
		}}, nil
	case 404:
		return &PollResult{Pending: true, Message: "已扫码，请在手机上确认登录"}, nil
	case 408:
		return &PollResult{Pending: true}, nil
	case 0:
		// An envelope without a status tells us nothing; keep waiting.
		return &PollResult{Pending: true}, nil
	case 402:
		return &PollResult{Expired: true, Message: "二维码已过期，请重新获取"}, nil
	case 403:
		return &PollResult{Expired: true, Message: "已在手机上取消登录，请重新获取二维码"}, nil
	default:
		return &PollResult{Expired: true, Message: fmt.Sprintf("登录失败：微信返回 %d", code)}, nil
	}
}

func (l *Login) setLast(v string) {
	l.lastMu.Lock()
	l.last = v
	l.lastMu.Unlock()
}

func (l *Login) getLast() string {
	l.lastMu.Lock()
	defer l.lastMu.Unlock()
	return l.last
}

// exchange trades the one-time WeChat code for API credentials.
func (l *Login) exchange(ctx context.Context, code string) (*Credentials, error) {
	deviceID := l.store.DeviceID()
	timestamp := time.Now().UnixMilli()
	random := int(time.Now().UnixNano()%1000) + 1
	payload := map[string]any{
		"appFirstInstall": 1,
		"code":            code,
		"deviceId":        deviceID,
		"deviceName":      device.Name,
		"installId":       l.store.InstallID(),
		"isAutoLogout":    0,
		"isFromQrcode":    1,
		"random":          random,
		"signature":       protocol.SHA256Hex(strconv.FormatInt(timestamp, 10) + deviceID + strconv.Itoa(random)),
		"timestamp":       timestamp,
		"trackId":         "",
		"deviceType":      device.Type,
	}

	resp, err := l.http.Do(ctx, httpx.Request{
		URL:    l.ep.login,
		Method: "POST",
		Body:   payload,
	})
	if err != nil {
		return nil, err
	}

	var out struct {
		VID          weread.FlexString `json:"vid"`
		AccessToken  string            `json:"accessToken"`
		RefreshToken string            `json:"refreshToken"`
		ErrCode      int               `json:"errCode"`
		ErrMsg       string            `json:"errMsg"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("%w: 登录接口返回了非 JSON 内容", weread.ErrProtocolChanged)
	}
	if resp.Status != 200 || out.AccessToken == "" || out.RefreshToken == "" || out.VID.String() == "" {
		msg := strings.TrimSpace(out.ErrMsg)
		if msg == "" {
			msg = "登录被拒绝"
		}
		return nil, fmt.Errorf("登录失败：%d %s", out.ErrCode, msg)
	}

	return l.complete(ctx, out.VID.String(), out.AccessToken, out.RefreshToken)
}

// complete persists the minted credentials and fills in the display identity.
func (l *Login) complete(ctx context.Context, vid, accessToken, refreshToken string) (*Credentials, error) {
	cookies := map[string]string{
		"wr_vid":  vid,
		"wr_skey": accessToken,
		"wr_fp":   l.store.Fingerprint(),
		"wr_ql":   "0",
	}
	// `wr_rt` is only meaningful for the browser session, which issues refresh
	// tokens prefixed with `web@`. Writing an E-Ink token there would make the
	// web renewal endpoint reject the cookie set.
	if strings.HasPrefix(refreshToken, "web@") {
		cookies["wr_rt"] = url.QueryEscape(refreshToken)
	}
	if err := l.http.SetCookies(webBase, cookies); err != nil {
		return nil, err
	}

	creds := &Credentials{
		VID:          vid,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Cookies:      cookies,
		DeviceID:     l.store.DeviceID(),
		Fingerprint:  l.store.Fingerprint(),
	}

	// Identity and the optional Skill API key are best-effort: a missing key
	// only disables the official gateway, not the two main channels.
	var user struct {
		Name   string `json:"name"`
		Nick   string `json:"nickname"`
		Avatar string `json:"avatar"`
		Photo  string `json:"photo"`
	}
	u := l.ep.user + "?userVid=" + url.QueryEscape(vid)
	if err := l.http.JSON(ctx, httpx.Request{
		URL:     u,
		Method:  "GET",
		Referer: webBase + "/",
		Headers: map[string]string{
			"User-Agent": protocol.DefaultUserAgent,
			"Accept":     "application/json, text/plain, */*",
		},
	}, &user); err == nil {
		creds.Name = firstNonEmpty(user.Name, user.Nick)
		creds.Avatar = firstNonEmpty(user.Avatar, user.Photo)
	}

	var key struct {
		APIKey string `json:"apikey"`
	}
	if err := l.http.JSON(ctx, httpx.Request{
		URL:     l.ep.apiKey,
		Method:  "GET",
		Referer: webBase + "/",
		Headers: map[string]string{
			"User-Agent": protocol.DefaultUserAgent,
			"Accept":     "application/json, text/plain, */*",
			"X-Vid":      vid,
			"X-Skey":     accessToken,
		},
	}, &key); err == nil {
		creds.APIKey = key.APIKey
	}

	if err := l.store.Save(creds); err != nil {
		return nil, err
	}
	return creds, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// RenewWeb refreshes the web session cookies.
func RenewWeb(ctx context.Context, c *httpx.Client, cookies map[string]string) (map[string]string, error) {
	if c.UserAgent == "" {
		c.UserAgent = protocol.DefaultUserAgent
	}
	if err := c.SetCookies(webBase, cookies); err != nil {
		return nil, err
	}
	body := map[string]any{"rq": "%2Fweb%2Fbook%2Fread", "ql": false}
	var out struct {
		Succ int `json:"succ"`
	}
	err := c.JSON(ctx, httpx.Request{
		URL:     webBase + "/web/login/renewal",
		Method:  "POST",
		Body:    body,
		Referer: webBase + "/",
		Origin:  webBase,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Succ != 1 {
		return nil, &weread.APIError{Endpoint: "login/renewal", Message: "会话续期被拒绝"}
	}
	merged := map[string]string{}
	for k, v := range cookies {
		merged[k] = v
	}
	for k, v := range c.Cookies(webBase) {
		merged[k] = v
	}
	return merged, nil
}

// RawJSON renders a body for debug logging without leaking secrets.
func RawJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(b))
}
