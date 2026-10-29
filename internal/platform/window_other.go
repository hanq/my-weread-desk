//go:build !windows

package platform

// Manager is a no-op implementation for platforms without the Win32 extras.
// The core reading experience works; only click-through, embedding and global
// hotkeys are unavailable.
type Manager struct{}

// New creates a stub manager.
func New() *Manager { return &Manager{} }

// SetOpacity is a no-op.
func (m *Manager) SetOpacity(float64) error { return nil }

// SetClickThrough is a no-op.
func (m *Manager) SetClickThrough(bool) error { return nil }

// ClickThrough always reports false.
func (m *Manager) ClickThrough() bool { return false }

// SetTopmost is a no-op.
func (m *Manager) SetTopmost(bool) error { return nil }

// Embed is unsupported.
func (m *Manager) Embed(string) error { return errUnsupported }

// Unembed is unsupported.
func (m *Manager) Unembed() error { return errUnsupported }

// Hotkey is one accelerator binding.
type Hotkey struct {
	Name        string `json:"name"`
	Accelerator string `json:"accelerator"`
}

// RegisterHotkeys is unsupported.
func (m *Manager) RegisterHotkeys([]Hotkey, func(string)) error { return errUnsupported }

// UnregisterHotkeys is a no-op.
func (m *Manager) UnregisterHotkeys() {}

type platformError string

func (e platformError) Error() string { return string(e) }

const errUnsupported = platformError("platform: 当前系统不支持该窗口能力")
