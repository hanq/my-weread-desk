package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubServer mimics the observed behaviour of the live WeChat QR handshake.
type stubServer struct {
	t *testing.T

	ticketRequests int
	qrRequests     int
	pollRequests   int
	loginRequests  int
	lastSeen       string
	loginBody      map[string]any

	// pollBodies is consumed in order; the last entry repeats.
	pollBodies []string
	// loginBodies is consumed in order; the last entry repeats.
	loginBodies []string
}

func (s *stubServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/wxticket", func(w http.ResponseWriter, r *http.Request) {
		s.ticketRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"signature":"sig-1","timeStamp":1793261603,"expires_in":460}`))
	})
	mux.HandleFunc("/connect/sdk/qrconnect", func(w http.ResponseWriter, r *http.Request) {
		s.qrRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"uuid":"uuid-1","qrcode":{"qrcodebase64":"/9j/4AAQSkZJRg=="}}`))
	})
	mux.HandleFunc("/connect/l/qrconnect", func(w http.ResponseWriter, r *http.Request) {
		s.pollRequests++
		s.lastSeen = r.URL.Query().Get("last")
		idx := s.pollRequests - 1
		if idx >= len(s.pollBodies) {
			idx = len(s.pollBodies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.pollBodies[idx]))
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		s.loginRequests++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.loginBody = body
		idx := s.loginRequests - 1
		if idx >= len(s.loginBodies) {
			idx = len(s.loginBodies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.loginBodies[idx]))
	})
	mux.HandleFunc("/api/userInfo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"测试用户","avatar":"https://img/x.png"}`))
	})
	mux.HandleFunc("/api/skills/apikeyGet", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"apikey":"key-123"}`))
	})
	return mux
}

func newTestLogin(t *testing.T, polls []string, logins []string) (*Login, *stubServer) {
	t.Helper()
	if len(logins) == 0 {
		logins = []string{`{"vid":58328018,"accessToken":"tok-1","refreshToken":"rt-1"}`}
	}
	stub := &stubServer{t: t, pollBodies: polls, loginBodies: logins}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	ep := endpoints{
		ticket: srv.URL + "/wxticket?nonceStr=weread",
		qr:     srv.URL + "/connect/sdk/qrconnect",
		poll:   srv.URL + "/connect/l/qrconnect",
		login:  srv.URL + "/login",
		user:   srv.URL + "/api/userInfo",
		apiKey: srv.URL + "/api/skills/apikeyGet?only_show=1",
	}
	return newLogin(store, ep), stub
}

func TestStartBuildsWeChatQRCode(t *testing.T) {
	login, stub := newTestLogin(t, []string{`{"wx_errcode":408}`}, nil)

	code, err := login.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if code.UID != "uuid-1" {
		t.Errorf("uid = %q", code.UID)
	}
	if !strings.Contains(code.Payload, "appid=wxab9b71ad2b90ff34") || !strings.Contains(code.Payload, "signature=sig-1") {
		t.Errorf("payload = %q", code.Payload)
	}
	if !strings.HasPrefix(code.Image, "data:image/jpeg;base64,") {
		t.Errorf("image prefix = %q", code.Image[:24])
	}
	if code.Expires <= time.Now().Unix() {
		t.Errorf("expiry = %d", code.Expires)
	}
	if stub.ticketRequests != 1 || stub.qrRequests != 1 {
		t.Errorf("ticket=%d qr=%d, want 1 each", stub.ticketRequests, stub.qrRequests)
	}
}

// 408 is the normal "nobody scanned yet" answer of a long poll.
func TestPollTimeoutCodeIsPending(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"wx_errcode":408}`}, nil)
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := login.Poll(context.Background(), "uuid-1", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Pending || res.Expired || res.Done || res.NeedOTP {
		t.Fatalf("result = %+v", res)
	}
}

func TestPollScannedKeepsWaiting(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"wx_errcode":404}`}, nil)
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	res, err := login.Poll(context.Background(), "uuid-1", "")
	if err != nil || !res.Pending {
		t.Fatalf("result = %+v err=%v", res, err)
	}
	if !strings.Contains(res.Message, "确认") {
		t.Errorf("message = %q", res.Message)
	}
}

func TestPollConfirmedExchangesCode(t *testing.T) {
	login, stub := newTestLogin(t, []string{`{"wx_errcode":405,"wx_code":"wx-code-1"}`}, nil)
	code, err := login.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := login.Poll(context.Background(), code.UID, "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Done {
		t.Fatalf("result = %+v", res)
	}
	if res.Account == nil || res.Account.VID != "58328018" || res.Account.Name != "测试用户" {
		t.Fatalf("account = %+v", res.Account)
	}
	if !res.Account.HasKey {
		t.Error("API key was not captured")
	}
	if stub.loginBody["code"] != "wx-code-1" {
		t.Errorf("login code = %v", stub.loginBody["code"])
	}
	if stub.loginBody["deviceId"] != login.store.DeviceID() {
		t.Errorf("deviceId = %v", stub.loginBody["deviceId"])
	}
	if _, ok := stub.loginBody["installId"]; !ok {
		t.Error("installId missing from the login body")
	}

	creds := login.store.Get()
	if creds == nil || creds.AccessToken != "tok-1" || creds.RefreshToken != "rt-1" {
		t.Fatalf("stored credentials = %+v", creds)
	}
	if creds.Cookies["wr_skey"] != "tok-1" || creds.Cookies["wr_vid"] != "58328018" {
		t.Errorf("cookies = %+v", creds.Cookies)
	}
	// The E-Ink refresh token must not be written to the browser cookie.
	if _, ok := creds.Cookies["wr_rt"]; ok {
		t.Errorf("wr_rt must stay empty for an E-Ink token, got %q", creds.Cookies["wr_rt"])
	}
}

// A rejected exchange must surface the server message instead of a bare status.
func TestPollExchangeFailureIsReported(t *testing.T) {
	login, _ := newTestLogin(t,
		[]string{`{"wx_errcode":405,"wx_code":"wx-code-1"}`},
		[]string{`{"errCode":-2013,"errMsg":"鉴权失败"}`},
	)
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	res, err := login.Poll(context.Background(), "uuid-1", "")
	if err == nil {
		t.Fatalf("expected an error, got %+v", res)
	}
	if !strings.Contains(err.Error(), "鉴权失败") {
		t.Errorf("error = %v", err)
	}
}

func TestPollExpiredCodesStopPolling(t *testing.T) {
	for _, body := range []string{`{"wx_errcode":402}`, `{"wx_errcode":403}`} {
		login, _ := newTestLogin(t, []string{body}, nil)
		if _, err := login.Start(context.Background()); err != nil {
			t.Fatalf("Start: %v", err)
		}
		res, err := login.Poll(context.Background(), "uuid-1", "")
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if !res.Expired || res.Pending {
			t.Fatalf("%s result = %+v", body, res)
		}
	}
}

// A non-JSON body must surface as an error so the UI backs off; it must never
// be mistaken for a pending poll or a reason to mint a new QR code.
func TestPollNonJSONBodyIsAnError(t *testing.T) {
	login, _ := newTestLogin(t, []string{`<html>risk control</html>`}, nil)
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	res, err := login.Poll(context.Background(), "uuid-1", "")
	if err == nil {
		t.Fatalf("expected an error, got %+v", res)
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("error = %v", err)
	}
}

func TestPollAfterSessionTimeoutExpires(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"wx_errcode":408}`}, nil)
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	login.startedAt = time.Now().Add(-2 * loginTimeout)
	res, err := login.Poll(context.Background(), "uuid-1", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Expired {
		t.Fatalf("result = %+v", res)
	}
}

func TestPollWithEmptyUIDExpires(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"wx_errcode":408}`}, nil)
	res, err := login.Poll(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Expired {
		t.Fatalf("result = %+v", res)
	}
}

// The previous status is replayed so WeChat can hold the next poll open instead
// of answering immediately.
func TestPollReplaysLastStatus(t *testing.T) {
	login, stub := newTestLogin(t, []string{`{"wx_errcode":404}`, `{"wx_errcode":408}`}, nil)
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := login.Poll(context.Background(), "uuid-1", ""); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if _, err := login.Poll(context.Background(), "uuid-1", ""); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if stub.lastSeen != "404" {
		t.Errorf("last = %q, want 404", stub.lastSeen)
	}
}

func TestCredentialsPersistEncrypted(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	creds := &Credentials{VID: "v", AccessToken: "a", RefreshToken: "r", Cookies: map[string]string{"wr_skey": "s"}}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.bin"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "wr_skey") || strings.Contains(string(raw), "\"a\"") {
		t.Error("credentials were written in plaintext")
	}

	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := reopened.Get()
	if got == nil || got.VID != "v" || got.Cookies["wr_skey"] != "s" {
		t.Fatalf("round trip = %+v", got)
	}
	// The device identity survives a logout.
	fp := reopened.Fingerprint()
	if err := reopened.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if reopened.Get() != nil {
		t.Error("credentials still present after Clear")
	}
	if reopened.Fingerprint() != fp {
		t.Error("fingerprint must survive logout")
	}
}

func TestFingerprintAndInstallIDShape(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	fp := store.Fingerprint()
	if fp == "" {
		t.Fatal("empty fingerprint")
	}
	for _, r := range fp {
		if r < '0' || r > '9' {
			t.Fatalf("fingerprint must be decimal, got %q", fp)
		}
	}
	if !strings.HasPrefix(store.DeviceID(), "eink") {
		t.Errorf("device id = %q", store.DeviceID())
	}
	installID := store.InstallID()
	if !strings.HasPrefix(installID, "eink31") || len(installID) != len("eink31")+26 {
		t.Errorf("install id = %q", installID)
	}
	if store.InstallID() != installID {
		t.Error("install id must be stable across calls")
	}
}
