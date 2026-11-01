//go:build !windows && !darwin

package platform

// setWindowIcon is a no-op on platforms without a dock or taskbar icon API.
func setWindowIcon(string) error { return nil }
