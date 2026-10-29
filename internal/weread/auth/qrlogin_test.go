package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubServer mimics the observed behaviour of the live QR handshake endpoints.
type stubServer struct {
	t *testing.T

	uidRequests     int
	infoRequests    int
	fingerprintSeen string

	// infoBodies is consumed in order; the last entry repeats.
	infoBodies []string
}

func (s *stubServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/r/weread-skills", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("wr_fp"); err == nil {
			s.fingerprintSeen = c.Value
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>skills</body></html>"))
	})
	mux.HandleFunc("/api/auth/getLoginUid", func(w http.ResponseWriter, r *http.Request) {
		s.uidRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uid":"uid-1"}`))
	})
	mux.HandleFunc("/api/auth/getLoginInfo", func(w http.ResponseWriter, r *http.Request) {
		s.infoRequests++
		idx := s.infoRequests - 1
		if idx >= len(s.infoBodies) {
			idx = len(s.infoBodies) - 1
		}
		body := s.infoBodies[idx]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/api/userInfo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"测试用户","avatar":"https://img/x.png"}`))
	})
	mux.HandleFunc("/api/skills/apikeyGet", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"apikey":"key-123"}`))
	})
	return mux
}

func newTestLogin(t *testing.T, bodies []string) (*Login, *stubServer) {
	t.Helper()
	stub := &stubServer{t: t, infoBodies: bodies}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	ep := endpoints{
		base:   srv.URL,
		skills: srv.URL + "/r/weread-skills",
		uid:    srv.URL + "/api/auth/getLoginUid",
		info:   srv.URL + "/api/auth/getLoginInfo",
		user:   srv.URL + "/api/userInfo",
		apiKey: srv.URL + "/api/skills/apikeyGet?only_show=1",
	}
	return newLogin(store, ep), stub
}

func TestStartLoginSendsFingerprintAndBuildsQRCode(t *testing.T) {
	login, stub := newTestLogin(t, []string{`{"succeed":false,"logicCode":"LOGIN_TIMEOUT"}`})

	code, err := login.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if code.UID != "uid-1" {
		t.Errorf("uid = %q", code.UID)
	}
	if !strings.Contains(code.Payload, "uid-1") {
		t.Errorf("payload = %q", code.Payload)
	}
	if !strings.HasPrefix(code.Image, "data:image/png;base64,") {
		t.Errorf("image prefix = %q", code.Image[:24])
	}
	if code.Expires <= time.Now().Unix() {
		t.Errorf("expiry = %d", code.Expires)
	}
	if stub.fingerprintSeen == "" {
		t.Error("the device fingerprint cookie was not sent")
	}
	if stub.uidRequests != 1 {
		t.Errorf("getLoginUid called %d times, want 1", stub.uidRequests)
	}
}

// LOGIN_TIMEOUT after the long poll means "not scanned yet" and must keep the
// caller polling the same UID instead of asking for a new QR code.
func TestPollLongPollTimeoutIsPending(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"succeed":false,"logicCode":"LOGIN_TIMEOUT","canRetry":false}`})
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := login.Poll(context.Background(), "uid-1", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Pending || res.Expired || res.Done || res.NeedOTP {
		t.Fatalf("result = %+v", res)
	}
}

func TestPollToleratesEmptyAndUnparseableBodies(t *testing.T) {
	login, _ := newTestLogin(t, []string{"", `{"succeed":1,"webLoginVid":"vid-9","accessToken":"tok","refreshToken":"ref"}`})
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := login.Poll(context.Background(), "uid-1", "")
	if err != nil || !res.Pending {
		t.Fatalf("empty body: %+v err=%v", res, err)
	}

	// `succeed` spelled as 1 must still complete the login.
	res, err = login.Poll(context.Background(), "uid-1", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Done {
		t.Fatalf("result = %+v", res)
	}
	if res.Account == nil || res.Account.VID != "vid-9" || res.Account.Name != "测试用户" {
		t.Fatalf("account = %+v", res.Account)
	}
	if !res.Account.HasKey {
		t.Error("API key was not captured")
	}
}

func TestPollNeedOTPAndMismatch(t *testing.T) {
	login, _ := newTestLogin(t, []string{
		`{"succeed":false,"logicCode":"NEED_OTP"}`,
		`{"succeed":false,"logicCode":"OTP_NOT_MATCH"}`,
	})
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := login.Poll(context.Background(), "uid-1", "")
	if err != nil || !res.NeedOTP {
		t.Fatalf("first poll = %+v err=%v", res, err)
	}
	res, err = login.Poll(context.Background(), "uid-1", "0000")
	if err != nil || !res.NeedOTP || res.Expired {
		t.Fatalf("mismatch poll = %+v err=%v", res, err)
	}
	if !strings.Contains(res.Message, "验证码") {
		t.Errorf("message = %q", res.Message)
	}
}

func TestPollExpiredStopsPolling(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"succeed":false,"logicCode":"LOGIN_CANCEL"}`})
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	res, err := login.Poll(context.Background(), "uid-1", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Expired || res.Pending {
		t.Fatalf("result = %+v", res)
	}
}

// A non-JSON body must surface as an error so the UI backs off; it must never
// be mistaken for a pending poll or a reason to mint a new QR code.
func TestPollNonJSONBodyIsAnError(t *testing.T) {
	login, _ := newTestLogin(t, []string{`<html>risk control</html>`})
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	res, err := login.Poll(context.Background(), "uid-1", "")
	if err == nil {
		t.Fatalf("expected an error, got %+v", res)
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("error = %v", err)
	}
}

func TestPollAfterSessionTimeoutExpires(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"succeed":false,"logicCode":""}`})
	if _, err := login.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	login.startedAt = time.Now().Add(-2 * loginTimeout)
	res, err := login.Poll(context.Background(), "uid-1", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Expired {
		t.Fatalf("result = %+v", res)
	}
}

func TestPollWithEmptyUIDExpires(t *testing.T) {
	login, _ := newTestLogin(t, []string{`{"succeed":false}`})
	res, err := login.Poll(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !res.Expired {
		t.Fatalf("result = %+v", res)
	}
}

func TestPollOmitsEmptyOTPParameter(t *testing.T) {
	seen := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/getLoginInfo", func(w http.ResponseWriter, r *http.Request) {
		seen <- r.URL.RawQuery
		_, _ = w.Write([]byte(`{"succeed":false,"logicCode":"LOGIN_TIMEOUT"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	ep := endpoints{base: srv.URL, skills: srv.URL, uid: srv.URL, info: srv.URL + "/api/auth/getLoginInfo", user: srv.URL, apiKey: srv.URL}
	login := newLogin(store, ep)
	login.startedAt = time.Now()

	if _, err := login.Poll(context.Background(), "u", ""); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	query := <-seen
	if strings.Contains(query, "otp") {
		t.Fatalf("empty otp parameter must be omitted, got %q", query)
	}
	if !strings.Contains(query, "uid=u") {
		t.Fatalf("uid missing from %q", query)
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

func TestFingerprintShape(t *testing.T) {
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
}
