// Package session owns the live credential state and the two transports that
// depend on it. Every channel shares one Session so a renewal in one place is
// immediately visible everywhere.
package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/auth"
	"my-weread-desk/internal/weread/device"
	"my-weread-desk/internal/weread/httpx"
	"my-weread-desk/internal/weread/protocol"
)

const (
	einkBase = "https://i.weread.qq.com"
	webBase  = "https://weread.qq.com"
)

// Session is the single source of truth for credentials.
type Session struct {
	mu    sync.RWMutex
	store *auth.Store
	creds *auth.Credentials

	webClient  *httpx.Client
	einkClient *httpx.Client

	refreshMu sync.Mutex
}

// New creates a session bound to a credential store.
func New(store *auth.Store) *Session {
	s := &Session{store: store, creds: store.Get()}

	web := httpx.New(true)
	web.UserAgent = protocol.DefaultUserAgent
	s.webClient = web

	eink := httpx.New(false)
	s.einkClient = eink
	s.applyEinkHeaders()

	if s.creds != nil {
		_ = web.SetCookies(webBase, s.creds.Cookies)
	}
	return s
}

// Store exposes the credential store.
func (s *Session) Store() *auth.Store { return s.store }

// Credentials returns a snapshot of the current credentials.
func (s *Session) Credentials() *auth.Credentials {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.creds
}

// LoggedIn reports whether a usable session exists.
func (s *Session) LoggedIn() bool {
	c := s.Credentials()
	return c != nil && (c.HasToken() || c.HasWebSession())
}

// Account returns the UI-facing account record.
func (s *Session) Account() *weread.Account {
	c := s.Credentials()
	if c == nil {
		return nil
	}
	return &weread.Account{
		Alias: "default", VID: c.VID, Name: c.Name, Avatar: c.Avatar,
		HasKey: c.APIKey != "", Updated: c.UpdatedAt,
	}
}

// SetCredentials installs freshly obtained credentials.
func (s *Session) SetCredentials(c *auth.Credentials) error {
	if c == nil {
		return errors.New("session: nil credentials")
	}
	s.mu.Lock()
	s.creds = c
	s.mu.Unlock()
	s.applyEinkHeaders()
	_ = s.webClient.SetCookies(webBase, c.Cookies)
	return s.store.Save(c)
}

// Clear forgets the account.
func (s *Session) Clear() error {
	s.mu.Lock()
	s.creds = nil
	s.mu.Unlock()
	s.applyEinkHeaders()
	return s.store.Clear()
}

// Web returns the cookie-backed client.
func (s *Session) Web() *httpx.Client { return s.webClient }

// Eink returns the token-backed client.
func (s *Session) Eink() *httpx.Client { return s.einkClient }

// WebBase and EinkBase expose the channel roots.
func WebBase() string  { return webBase }
func EinkBase() string { return einkBase }

func (s *Session) applyEinkHeaders() {
	s.einkClient.UserAgent = device.UserAgent
	for k, v := range device.Headers() {
		s.einkClient.SetHeader(k, v)
	}
	if c := s.creds; c != nil && c.HasToken() {
		s.einkClient.SetHeader("vid", c.VID)
		s.einkClient.SetHeader("accessToken", c.AccessToken)
	} else {
		s.einkClient.SetHeader("vid", "")
		s.einkClient.SetHeader("accessToken", "")
	}
}

// RefreshEink mints a new access token from the refresh token.
func (s *Session) RefreshEink(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.refreshEinkLocked(ctx)
}

func (s *Session) refreshEinkLocked(ctx context.Context) error {
	c := s.Credentials()
	if c == nil || c.RefreshToken == "" || c.VID == "" {
		return weread.ErrAuthRequired
	}
	// The browser login issues `web@...` refresh tokens, which /login rejects
	// with -2013. Refreshing them can never succeed, so tell the user what to
	// do instead of burning a request on it.
	if strings.HasPrefix(c.RefreshToken, "web@") {
		return weread.ErrLegacyLogin
	}

	timestamp := time.Now().UnixMilli()
	random := int(time.Now().UnixNano()%1000) + 1
	body := map[string]any{
		"deviceId":     c.DeviceID,
		"deviceName":   device.Name,
		"inBackground": 0,
		"kickType":     1,
		"random":       random,
		"refCgi":       "",
		"refreshToken": c.RefreshToken,
		"signature":    protocol.SHA256Hex(strconv.FormatInt(timestamp, 10) + c.DeviceID + strconv.Itoa(random)),
		"timestamp":    timestamp,
		"trackId":      "",
		"deviceType":   device.Type,
	}
	var out struct {
		VID          any    `json:"vid"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ErrCode      int    `json:"errCode"`
		ErrMsg       string `json:"errMsg"`
	}
	if err := s.einkClient.JSON(ctx, httpx.Request{
		URL:    einkBase + "/login",
		Method: "POST",
		Body:   body,
	}, &out); err != nil {
		return err
	}
	if out.AccessToken == "" || out.VID == nil {
		return fmt.Errorf("令牌续期失败: %s", out.ErrMsg)
	}

	next := *c
	next.AccessToken = out.AccessToken
	if out.RefreshToken != "" {
		next.RefreshToken = out.RefreshToken
	}
	next.VID = fmt.Sprint(out.VID)
	next.Cookies = map[string]string{}
	for k, v := range c.Cookies {
		next.Cookies[k] = v
	}
	next.Cookies["wr_vid"] = next.VID
	next.Cookies["wr_skey"] = next.AccessToken

	s.mu.Lock()
	s.creds = &next
	s.mu.Unlock()
	s.applyEinkHeaders()
	_ = s.webClient.SetCookies(webBase, next.Cookies)
	return s.store.Save(&next)
}

// RefreshWeb renews the cookie session. The browser renewal endpoint needs the
// `wr_rt` cookie, which only the browser login issues; credentials minted by
// the E-Ink login have no such cookie, so the API token is refreshed instead
// and `wr_skey` is reseeded from it.
func (s *Session) RefreshWeb(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	c := s.Credentials()
	if c == nil || !c.HasWebSession() {
		return weread.ErrAuthRequired
	}
	merged, err := auth.RenewWeb(ctx, s.webClient, c.Cookies)
	if err != nil {
		if rerr := s.refreshEinkLocked(ctx); rerr != nil {
			return err
		}
		return nil
	}
	next := *c
	next.Cookies = merged
	s.mu.Lock()
	s.creds = &next
	s.mu.Unlock()
	return s.store.Save(&next)
}

// MergeCookies persists cookies set by any web response.
func (s *Session) MergeCookies(cookies map[string]string) {
	if len(cookies) == 0 {
		return
	}
	c := s.Credentials()
	if c == nil {
		return
	}
	changed := false
	next := *c
	next.Cookies = make(map[string]string, len(c.Cookies)+len(cookies))
	for k, v := range c.Cookies {
		next.Cookies[k] = v
	}
	for k, v := range cookies {
		if next.Cookies[k] != v {
			next.Cookies[k] = v
			changed = true
		}
	}
	if !changed {
		return
	}
	s.mu.Lock()
	s.creds = &next
	s.mu.Unlock()
	_ = s.store.Save(&next)
}

// CookieSnapshot returns the current cookies.
func (s *Session) CookieSnapshot() map[string]string {
	c := s.Credentials()
	if c == nil {
		return map[string]string{}
	}
	return c.Cookies
}
