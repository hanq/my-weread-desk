package weread

import (
	"errors"
	"fmt"
	"strings"
)

// Error codes returned by WeRead.
const (
	ErrLoginTimeout = -2012
	ErrNoSuchUser   = -2010
	ErrNeedLogin    = -2011
)

// APIError is a business-level failure returned by a WeRead endpoint.
type APIError struct {
	Endpoint string
	Code     int
	Message  string
	Status   int
}

func (e *APIError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "未知错误"
	}
	if e.Code == 0 && e.Status != 0 {
		return fmt.Sprintf("微信读书接口 %s 返回 HTTP %d", e.Endpoint, e.Status)
	}
	return fmt.Sprintf("微信读书接口 %s 返回 %d：%s", e.Endpoint, e.Code, msg)
}

// IsAuthExpired reports whether the error means the session must be renewed
// (or the user must log in again).
func IsAuthExpired(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == ErrLoginTimeout || apiErr.Code == ErrNeedLogin || apiErr.Status == 401
	}
	return false
}

// IsNoSuchUser reports the -2010 identity mismatch. A stale or malformed vid
// produces this code and can be repaired by a token refresh, so the E-Ink
// client treats it like an expired session once.
func IsNoSuchUser(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == ErrNoSuchUser
}

// ErrNotEntitled is returned when the account may not read a chapter.
var ErrNotEntitled = errors.New("当前账号无权阅读该章节")

// ErrAuthRequired is returned when no usable credential exists.
var ErrAuthRequired = errors.New("需要登录微信读书")

// ErrProtocolChanged is returned when a response no longer matches the known
// shape. It is surfaced verbatim so users can tell a protocol change apart
// from a network failure.
var ErrProtocolChanged = errors.New("微信读书接口格式已变化，请更新应用")

// ErrLegacyLogin explains that the stored credentials came from the browser
// login. Their access token is rejected by i.weread.qq.com (-2012) and their
// `web@...` refresh token by /login (-2013), so the only fix is a fresh scan.
var ErrLegacyLogin = errors.New("当前登录凭据来自旧版网页登录，请退出登录后重新扫码")
