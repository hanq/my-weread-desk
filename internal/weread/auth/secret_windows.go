//go:build windows

package auth

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// protect seals data with the current Windows user's DPAPI key.
func protect(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("auth: empty payload")
	}
	in := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	buf := unsafe.Slice(out.Data, out.Size)
	sealed := make([]byte, len(buf))
	copy(sealed, buf)
	return append([]byte("DPAPI1"), sealed...), nil
}

// unprotect reverses protect.
func unprotect(sealed []byte) ([]byte, error) {
	if len(sealed) < 7 || string(sealed[:6]) != "DPAPI1" {
		return nil, errors.New("auth: not a DPAPI payload")
	}
	payload := sealed[6:]
	if len(payload) == 0 {
		return nil, errors.New("auth: empty payload")
	}
	in := windows.DataBlob{Size: uint32(len(payload)), Data: &payload[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	buf := unsafe.Slice(out.Data, out.Size)
	plain := make([]byte, len(buf))
	copy(plain, buf)
	return plain, nil
}
