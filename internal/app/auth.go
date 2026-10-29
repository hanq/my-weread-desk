package app

import (
	"context"
	"errors"
	"time"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/auth"
)

// AuthState is what the login gate renders.
type AuthState struct {
	LoggedIn bool            `json:"loggedIn"`
	Account  *weread.Account `json:"account,omitempty"`
	HasKey   bool            `json:"hasApiKey"`
}

// GetAuthState reports whether an account is available.
func (a *App) GetAuthState() AuthState {
	acc := a.session.Account()
	return AuthState{LoggedIn: a.session.LoggedIn(), Account: acc, HasKey: acc != nil && acc.HasKey}
}

// StartLogin creates a QR challenge.
func (a *App) StartLogin() (*weread.QRCode, error) {
	a.mu.Lock()
	if a.login == nil {
		a.login = auth.NewLogin(a.session.Store())
	}
	login := a.login
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	return login.Start(ctx)
}

// LoginPoll is the UI-facing poll result. `expired` tells the renderer to stop
// polling and wait for the user: the client never re-requests a QR code by
// itself, which is what keeps the login flow from turning into a request storm.
type LoginPoll struct {
	Pending bool            `json:"pending"`
	NeedOTP bool            `json:"needOtp"`
	Done    bool            `json:"done"`
	Expired bool            `json:"expired"`
	Message string          `json:"message"`
	Account *weread.Account `json:"account,omitempty"`
}

// PollLogin advances the QR login state machine.
func (a *App) PollLogin(uid, otp string) (*LoginPoll, error) {
	a.mu.Lock()
	login := a.login
	a.mu.Unlock()
	if login == nil {
		return nil, errors.New("登录会话不存在，请重新获取二维码")
	}
	// The poll holds the connection open for up to ~55s server side, so the
	// deadline must sit above that. A shorter one would abort every long poll
	// and make the client re-request immediately.
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 90*time.Second)
	defer cancel()
	res, err := login.Poll(ctx, uid, otp)
	if err != nil {
		return nil, err
	}
	if res.Done {
		// The login helper already persisted credentials; reload them into the
		// live session so the very next request is authenticated.
		if c := a.session.Store().Get(); c != nil {
			_ = a.session.SetCredentials(c)
		}
		go a.warmUpAfterLogin()
	}
	return &LoginPoll{
		Pending: res.Pending, NeedOTP: res.NeedOTP, Done: res.Done,
		Expired: res.Expired, Message: res.Message, Account: res.Account,
	}, nil
}

// warmUpAfterLogin refreshes the shelf in the background so the first screen
// is populated by the time the user looks at it.
func (a *App) warmUpAfterLogin() {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 45*time.Second)
	defer cancel()
	if _, err := a.syncShelf(ctx); err != nil {
		a.logf("登录后同步书架失败：%v", err)
	}
}

// Logout drops the account but keeps device identity.
func (a *App) Logout() error {
	a.mu.Lock()
	a.readerStates = map[string]*weread.ReaderState{}
	a.login = nil
	a.mu.Unlock()
	return a.session.Clear()
}

// RenewSession forces a token + cookie refresh.
func (a *App) RenewSession() error {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	if err := a.session.RefreshEink(ctx); err != nil {
		if werr := a.session.RefreshWeb(ctx); werr != nil {
			return err
		}
	}
	return a.session.RefreshWeb(ctx)
}

func (a *App) ctxOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
