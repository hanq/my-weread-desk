//go:build !windows

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// protect seals data with a locally stored AES-256-GCM key. This is weaker
// than an OS keystore but keeps the same at-rest guarantees across platforms.
func protect(plain []byte) ([]byte, error) {
	key, err := localKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return append([]byte("AESGCM"), gcm.Seal(nonce, nonce, plain, nil)...), nil
}

func unprotect(sealed []byte) ([]byte, error) {
	if len(sealed) < 7 || string(sealed[:6]) != "AESGCM" {
		return nil, errors.New("auth: unknown payload format")
	}
	key, err := localKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	payload := sealed[6:]
	if len(payload) < gcm.NonceSize() {
		return nil, errors.New("auth: truncated payload")
	}
	nonce, body := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	return gcm.Open(nil, nonce, body, nil)
}

func localKey() ([]byte, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "my-weread-desk", "secret.key")
	if raw, err := os.ReadFile(path); err == nil {
		key, derr := hex.DecodeString(string(raw))
		if derr == nil && len(key) == 32 {
			return key, nil
		}
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}
