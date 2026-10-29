package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Credentials is everything needed to talk to both WeRead channels.
type Credentials struct {
	VID          string            `json:"vid"`
	Name         string            `json:"name"`
	Avatar       string            `json:"avatar"`
	AccessToken  string            `json:"accessToken"`
	RefreshToken string            `json:"refreshToken"`
	APIKey       string            `json:"apiKey"`
	Cookies      map[string]string `json:"cookies"`
	DeviceID     string            `json:"deviceId"`
	Fingerprint  string            `json:"fingerprint"`
	UpdatedAt    int64             `json:"updatedAt"`
}

// HasWebSession reports whether the cookie set can serve content requests.
func (c *Credentials) HasWebSession() bool {
	return c != nil && c.Cookies["wr_skey"] != "" && c.Cookies["wr_vid"] != ""
}

// HasToken reports whether the E-Ink channel can be used.
func (c *Credentials) HasToken() bool {
	return c != nil && c.VID != "" && c.AccessToken != ""
}

// Store persists credentials on disk, encrypting the payload with the OS
// keystore when available.
type Store struct {
	mu       sync.RWMutex
	dir      string
	creds    *Credentials
	deviceID string
	fp       string
}

// NewStore loads credentials (if any) from dir.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.bin"))
	if err == nil {
		if plain, derr := unprotect(raw); derr == nil {
			var c Credentials
			if json.Unmarshal(plain, &c) == nil {
				s.creds = &c
			}
		}
	}
	s.deviceID = s.loadOrCreateDeviceID()
	s.fp = s.loadOrCreateFingerprint()
	if s.creds != nil {
		if s.creds.DeviceID == "" {
			s.creds.DeviceID = s.deviceID
		}
		if s.creds.Fingerprint == "" {
			s.creds.Fingerprint = s.fp
		}
	}
	return s, nil
}

// Dir returns the directory holding credentials.
func (s *Store) Dir() string { return s.dir }

// Get returns a copy of the current credentials, or nil.
func (s *Store) Get() *Credentials {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.creds == nil {
		return nil
	}
	cp := *s.creds
	cp.Cookies = make(map[string]string, len(s.creds.Cookies))
	for k, v := range s.creds.Cookies {
		cp.Cookies[k] = v
	}
	return &cp
}

// Save replaces the stored credentials.
func (s *Store) Save(c *Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c == nil {
		c = &Credentials{}
	}
	if c.DeviceID == "" {
		c.DeviceID = s.deviceID
	}
	if c.Fingerprint == "" {
		c.Fingerprint = s.fp
	}
	c.UpdatedAt = time.Now().Unix()
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	sealed, err := protect(raw)
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "credentials.bin")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, sealed, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	s.creds = c
	return nil
}

// Clear removes stored credentials but keeps the device identity so that a
// re-login does not look like a new device to the server.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds = nil
	err := os.Remove(filepath.Join(s.dir, "credentials.bin"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// DeviceID is a stable E-Ink style device identifier.
func (s *Store) DeviceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deviceID
}

// Fingerprint is the browser fingerprint cookie value.
func (s *Store) Fingerprint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fp
}

const devicePrefix = "eink334691225"

func (s *Store) loadOrCreateDeviceID() string {
	path := filepath.Join(s.dir, "device.id")
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 10 {
		return string(raw)
	}
	// A 19-digit suffix keeps the shape of the reference client.
	n, _ := rand.Int(rand.Reader, big.NewInt(9_000_000_000_000_000_000))
	id := fmt.Sprintf("%s%019d", devicePrefix, n)
	_ = os.WriteFile(path, []byte(id), 0o600)
	return id
}

func (s *Store) loadOrCreateFingerprint() string {
	path := filepath.Join(s.dir, "fingerprint")
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		return string(raw)
	}
	// Four OS-random bytes formatted as an unsigned decimal, matching the
	// browser fingerprint the official site issues.
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	value := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	fp := fmt.Sprintf("%d", value)
	_ = os.WriteFile(path, []byte(fp), 0o600)
	return fp
}
