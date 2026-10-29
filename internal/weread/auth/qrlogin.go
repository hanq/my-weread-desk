package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/httpx"
	"my-weread-desk/internal/weread/protocol"
)

const (
	baseURL = "https://weread.qq.com"

	loginTimeout = 5 * time.Minute
)

// endpoints groups the handshake URLs so tests can point them at a stub server.
type endpoints struct {
	base   string
	skills string
	uid    string
	info   string
	user   string
	apiKey string
}

func defaultEndpoints() endpoints {
	b := baseURL
	return endpoints{
		base:   b,
		skills: b + "/r/weread-skills",
		uid:    b + "/api/auth/getLoginUid",
		info:   b + "/api/auth/getLoginInfo",
		user:   b + "/api/userInfo",
		apiKey: b + "/api/skills/apikeyGet?only_show=1",
	}
}

// Login drives the WeChat QR login handshake.
type Login struct {
	http  *httpx.Client
	store *Store
	ep    endpoints

	startedAt time.Time
	uid       string
	cookies   map[string]string
}

// NewLogin creates a login helper bound to a credential store.
func NewLogin(store *Store) *Login {
	return newLogin(store, defaultEndpoints())
}

func newLogin(store *Store, ep endpoints) *Login {
	c := httpx.New(true)
	c.UserAgent = protocol.DefaultUserAgent
	return &Login{http: c, store: store, ep: ep, cookies: map[string]string{}}
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

// Start opens a login page, obtains a UID and renders it as a QR code.
func (l *Login) Start(ctx context.Context) (*weread.QRCode, error) {
	l.startedAt = time.Now()
	l.cookies = map[string]string{"wr_fp": l.store.Fingerprint()}
	if err := l.http.SetCookies(l.ep.base, l.cookies); err != nil {
		return nil, err
	}

	if _, err := l.http.Text(ctx, httpx.Request{
		URL:     l.ep.skills,
		Method:  "GET",
		Referer: l.ep.base + "/",
		Headers: map[string]string{
			"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		},
	}); err != nil {
		// The page is only needed to seed cookies; a failure here is not fatal
		// as long as getLoginUid still works.
		_ = err
	}
	l.mergeJar()

	var out struct {
		UID string `json:"uid"`
	}
	if err := l.http.JSON(ctx, httpx.Request{
		URL:     l.ep.uid,
		Method:  "GET",
		Referer: l.ep.skills,
		Headers: map[string]string{"Accept": "application/json, text/plain, */*"},
	}, &out); err != nil {
		return nil, err
	}
	if out.UID == "" {
		return nil, errors.New("微信读书未返回登录标识，请稍后重试")
	}
	l.mergeJar()
	l.uid = out.UID

	payload := l.ep.base + "/web/confirm?uid=" + url.QueryEscape(out.UID)
	png, err := qrcode.Encode(payload, qrcode.Medium, 512)
	if err != nil {
		return nil, err
	}
	return &weread.QRCode{
		UID:     out.UID,
		Payload: payload,
		Image:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		Expires: time.Now().Add(loginTimeout).Unix(),
	}, nil
}

// Poll checks the login state once.
//
// Protocol notes, verified against the live service:
//   - With no `otp` parameter the server holds the request open for roughly
//     55 seconds and then answers `{"succeed":false,"logicCode":"LOGIN_TIMEOUT"}`.
//     That is the normal "nobody has scanned yet" outcome, not an expired code,
//     so it maps to Pending.
//   - An empty `otp=` parameter is rejected immediately; it must be omitted.
//   - The request is never retried: a retry would double the request rate for
//     no benefit and is exactly what triggers risk control.
func (l *Login) Poll(ctx context.Context, uid, otp string) (*PollResult, error) {
	if uid == "" {
		return &PollResult{Expired: true, Message: "登录会话无效，请重新获取二维码"}, nil
	}
	if !l.startedAt.IsZero() && time.Since(l.startedAt) > loginTimeout {
		return &PollResult{Expired: true, Message: "二维码已过期，请重新获取"}, nil
	}

	u := l.ep.info + "?uid=" + url.QueryEscape(uid)
	// The long poll needs a timeout above the server's ~55s hold.
	timeout := 75 * time.Second
	if otp != "" {
		u += "&otp=" + url.QueryEscape(otp)
		timeout = 25 * time.Second
	}

	resp, err := l.http.Do(ctx, httpx.Request{
		URL:     u,
		Method:  "GET",
		Referer: l.ep.skills,
		Timeout: timeout,
		NoRetry: true,
		Headers: map[string]string{"Accept": "application/json, text/plain, */*"},
	})
	if err != nil {
		// A cancelled or timed-out long poll carries no information: the
		// session is still valid, so keep polling instead of erroring out.
		if ctx.Err() != nil {
			return &PollResult{Expired: true, Message: "登录已取消"}, nil
		}
		return &PollResult{Pending: true}, nil
	}
	l.mergeJar()

	if resp.Status != 200 {
		// 401/403 here means the login session was rejected; anything else is
		// transient. Both are reported as pending so the caller backs off
		// rather than restarting the handshake.
		return &PollResult{Pending: true}, nil
	}
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return &PollResult{Pending: true}, nil
	}
	if err := httpx.CheckEnvelope("getLoginInfo", resp.Status, resp.Body); err != nil {
		return &PollResult{Pending: true}, nil
	}

	var body struct {
		Succeed      weread.FlexBool   `json:"succeed"`
		WebLoginVID  weread.FlexString `json:"webLoginVid"`
		AccessToken  string            `json:"accessToken"`
		RefreshToken string            `json:"refreshToken"`
		LogicCode    string            `json:"logicCode"`
		CanRetry     weread.FlexBool   `json:"canRetry"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("%w: getLoginInfo 返回了非 JSON 内容", weread.ErrProtocolChanged)
	}

	if body.Succeed.Bool() {
		creds, err := l.complete(ctx, &loginPayload{
			Succeed:      true,
			WebLoginVID:  body.WebLoginVID.String(),
			AccessToken:  body.AccessToken,
			RefreshToken: body.RefreshToken,
			LogicCode:    body.LogicCode,
		})
		if err != nil {
			return nil, err
		}
		return &PollResult{Done: true, Message: "登录成功", Account: &weread.Account{
			VID: creds.VID, Name: creds.Name, Avatar: creds.Avatar, HasKey: creds.APIKey != "",
		}}, nil
	}

	switch strings.ToUpper(strings.TrimSpace(body.LogicCode)) {
	case "NEED_OTP":
		return &PollResult{NeedOTP: true, Message: "请输入手机上的四位验证码"}, nil
	case "OTP_NOT_MATCH":
		return &PollResult{NeedOTP: true, Message: "验证码不正确，请重新输入"}, nil
	case "OTP_EXPIRED":
		return &PollResult{NeedOTP: true, Message: "验证码已过期，请重新输入"}, nil
	case "LOGIN_TIMEOUT":
		// Still waiting for a scan: poll the same UID again.
		return &PollResult{Pending: true}, nil
	case "LOGIN_CANCEL":
		return &PollResult{Expired: true, Message: "登录已取消，请重新获取二维码"}, nil
	case "":
		return &PollResult{Pending: true}, nil
	default:
		return &PollResult{Expired: true, Message: "登录失败：" + body.LogicCode}, nil
	}
}

type loginPayload struct {
	Succeed      bool
	WebLoginVID  string
	AccessToken  string
	RefreshToken string
	LogicCode    string
}

func (l *Login) complete(ctx context.Context, body *loginPayload) (*Credentials, error) {
	cookies := map[string]string{}
	for k, v := range l.cookies {
		cookies[k] = v
	}
	cookies["wr_vid"] = body.WebLoginVID
	cookies["wr_skey"] = body.AccessToken
	cookies["wr_ql"] = "0"
	if body.RefreshToken != "" {
		cookies["wr_rt"] = url.QueryEscape(body.RefreshToken)
	}
	if err := l.http.SetCookies(l.ep.base, cookies); err != nil {
		return nil, err
	}

	creds := &Credentials{
		VID:          body.WebLoginVID,
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
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
	u := l.ep.user + "?userVid=" + url.QueryEscape(body.WebLoginVID)
	if err := l.http.JSON(ctx, httpx.Request{
		URL:     u,
		Method:  "GET",
		Referer: l.ep.skills,
		Headers: map[string]string{
			"Accept": "application/json, text/plain, */*",
			"X-Vid":  body.WebLoginVID,
			"X-Skey": body.AccessToken,
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
		Referer: l.ep.skills,
		Headers: map[string]string{
			"Accept": "application/json, text/plain, */*",
			"X-Vid":  body.WebLoginVID,
			"X-Skey": body.AccessToken,
		},
	}, &key); err == nil {
		creds.APIKey = key.APIKey
	}

	if err := l.store.Save(creds); err != nil {
		return nil, err
	}
	return creds, nil
}

func (l *Login) mergeJar() {
	for k, v := range l.http.Cookies(l.ep.base) {
		l.cookies[k] = v
	}
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
	if err := c.SetCookies(baseURL, cookies); err != nil {
		return nil, err
	}
	body := map[string]any{"rq": "%2Fweb%2Fbook%2Fread", "ql": false}
	var out struct {
		Succ int `json:"succ"`
	}
	err := c.JSON(ctx, httpx.Request{
		URL:     baseURL + "/web/login/renewal",
		Method:  "POST",
		Body:    body,
		Referer: "https://weread.qq.com/",
		Origin:  "https://weread.qq.com",
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
	for k, v := range c.Cookies(baseURL) {
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
