// Package httpx provides the HTTP plumbing shared by every WeRead channel:
// a cookie jar, JSON helpers, error-envelope detection and conservative
// retries.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"my-weread-desk/internal/weread"
)

const (
	defaultTimeout = 25 * time.Second
	maxBodyBytes   = 32 << 20
)

// Client wraps http.Client with a shared cookie jar and JSON helpers.
type Client struct {
	HTTP      *http.Client
	UserAgent string

	mu    sync.RWMutex
	extra map[string]string
}

// New creates a client. When useJar is true a cookie jar is installed so the
// web channel keeps its session across calls.
func New(useJar bool) *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	hc := &http.Client{Transport: transport, Timeout: 0}
	if useJar {
		jar, _ := cookiejar.New(nil)
		hc.Jar = jar
	}
	return &Client{HTTP: hc, UserAgent: "", extra: map[string]string{}}
}

// SetHeader sets a header applied to every request.
func (c *Client) SetHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if value == "" {
		delete(c.extra, key)
		return
	}
	c.extra[key] = value
}

// SetCookies loads a cookie map into the jar for the given host.
//
// The domain is derived from the URL rather than hard-coded: a cookie scoped to
// ".weread.qq.com" is required for the CDN host that serves chapter images, but
// forcing that domain onto every host would make the jar drop cookies for any
// other origin (a test stub, a mirror).
func (c *Client) SetCookies(rawURL string, cookies map[string]string) error {
	if c.HTTP.Jar == nil {
		return errors.New("httpx: cookie jar disabled")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	domain := cookieDomain(u.Hostname())
	list := make([]*http.Cookie, 0, len(cookies))
	for k, v := range cookies {
		ck := &http.Cookie{Name: k, Value: v, Path: "/"}
		if domain != "" {
			ck.Domain = domain
		}
		list = append(list, ck)
	}
	c.HTTP.Jar.SetCookies(u, list)
	return nil
}

// cookieDomain returns the domain attribute to use, or "" for a host-only
// cookie. IP addresses and single-label hosts cannot take a domain attribute.
func cookieDomain(host string) string {
	if host == "" || net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return ""
	}
	if strings.Count(host, ".") < 2 {
		return ""
	}
	return "." + host
}

// Cookies returns the current jar contents for the host.
func (c *Client) Cookies(rawURL string) map[string]string {
	out := map[string]string{}
	if c.HTTP.Jar == nil {
		return out
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return out
	}
	for _, ck := range c.HTTP.Jar.Cookies(u) {
		out[ck.Name] = ck.Value
	}
	return out
}

// Request is one HTTP call.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    any
	// RawBody bypasses JSON encoding.
	RawBody []byte
	// Referer and Origin are convenience fields for web endpoints.
	Referer string
	Origin  string
	// Timeout overrides the default.
	Timeout time.Duration
	// NoRetry disables the single transport retry (used for writes).
	NoRetry bool
}

// Response holds the raw response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Do performs the request, retrying once on transport errors for safe calls.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	attempts := 2
	if req.NoRetry {
		attempts = 1
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(600 * time.Millisecond):
			}
		}
		resp, err := c.doOnce(ctx, req, timeout)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// Never retry a cancelled context.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (c *Client) doOnce(ctx context.Context, req Request, timeout time.Duration) (*Response, error) {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	switch {
	case len(req.RawBody) > 0:
		bodyReader = bytes.NewReader(req.RawBody)
	case req.Body != nil:
		encoded, err := json.Marshal(req.Body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(encoded)
	}

	sub, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r, err := http.NewRequestWithContext(sub, method, req.URL, bodyReader)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	for k, v := range c.extra {
		r.Header.Set(k, v)
	}
	c.mu.RUnlock()

	if c.UserAgent != "" {
		r.Header.Set("User-Agent", c.UserAgent)
	}
	if req.Body != nil && len(req.RawBody) == 0 {
		r.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}
	if req.Referer != "" {
		r.Header.Set("Referer", req.Referer)
	}
	if req.Origin != "" {
		r.Header.Set("Origin", req.Origin)
	}
	for k, v := range req.Headers {
		if v == "" {
			r.Header.Del(k)
			continue
		}
		r.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(r)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", shortURL(req.URL), unwrapNetErr(err))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: data}, nil
}

func unwrapNetErr(err error) error {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.New("网络超时")
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s", urlErr.Err)
	}
	return err
}

func shortURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Host + u.Path
}

// Envelope is the common WeRead business-error wrapper.
type Envelope struct {
	ErrCode *int   `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
}

// CheckEnvelope returns an *weread.APIError when the body carries errCode.
func CheckEnvelope(endpoint string, status int, body []byte) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		if status >= 400 {
			return &weread.APIError{Endpoint: endpoint, Status: status}
		}
		return nil
	}
	var env Envelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil
	}
	if env.ErrCode != nil && *env.ErrCode != 0 {
		return &weread.APIError{Endpoint: endpoint, Code: *env.ErrCode, Message: env.ErrMsg, Status: status}
	}
	if status == http.StatusUnauthorized {
		return &weread.APIError{Endpoint: endpoint, Code: weread.ErrNeedLogin, Message: "登录已失效", Status: status}
	}
	return nil
}

// JSON performs a request and decodes the JSON response into out.
func (c *Client) JSON(ctx context.Context, req Request, out any) error {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	if err := CheckEnvelope(shortURL(req.URL), resp.Status, resp.Body); err != nil {
		return err
	}
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		if out != nil {
			return fmt.Errorf("%w: 空响应", weread.ErrProtocolChanged)
		}
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		if out == nil {
			return nil
		}
		return fmt.Errorf("%w: %v", weread.ErrProtocolChanged, err)
	}
	return nil
}

// Text performs a request and returns the body as a string, checking errors.
func (c *Client) Text(ctx context.Context, req Request) (string, error) {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return "", err
	}
	if err := CheckEnvelope(shortURL(req.URL), resp.Status, resp.Body); err != nil {
		return "", err
	}
	return string(resp.Body), nil
}

// DecodeJSON decodes a body that may be wrapped in {"data": ...}.
func DecodeJSON(body []byte, out any) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return fmt.Errorf("%w: 空响应", weread.ErrProtocolChanged)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: %v", weread.ErrProtocolChanged, err)
	}
	return nil
}

// JoinQuery builds a query string from ordered pairs.
func JoinQuery(pairs ...string) string {
	return strings.Join(pairs, "&")
}
