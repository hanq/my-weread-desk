//go:build windows

// Package platform isolates the Win32 window excursions: click-through,
// whole-window opacity, embedding and global hotkeys. Everything here is
// best-effort and must never crash the app when the OS refuses.
package platform

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shcore   = windows.NewLazySystemDLL("shcore.dll")

	procEnumWindows            = user32.NewProc("EnumWindows")
	procGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible        = user32.NewProc("IsWindowVisible")
	procGetWindowLongPtr       = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr       = user32.NewProc("SetWindowLongPtrW")
	procSetLayeredWindowAttrs  = user32.NewProc("SetLayeredWindowAttributes")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procSetParent              = user32.NewProc("SetParent")
	procRegisterHotKey         = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey       = user32.NewProc("UnregisterHotKey")
	procGetMessage             = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessage        = user32.NewProc("DispatchMessageW")
	procPeekMessage            = user32.NewProc("PeekMessageW")
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procEnumFontFamiliesEx     = gdi32.NewProc("EnumFontFamiliesExW")
	procClientToScreen         = user32.NewProc("ClientToScreen")
	procGetWindowRect          = user32.NewProc("GetWindowRect")
	procGetClassName           = user32.NewProc("GetClassNameW")
	procIsIconic               = user32.NewProc("IsIconic")
	procEnumDisplayMonitors    = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfo         = user32.NewProc("GetMonitorInfoW")
	procMonitorFromPoint       = user32.NewProc("MonitorFromPoint")
	procGetDpiForMonitor       = shcore.NewProc("GetDpiForMonitor")

	procGetCurrentProcessID = kernel32.NewProc("GetCurrentProcessId")
)

const (
	gwlExStyle      = uintptr(0xFFFFFFFFFFFFFFEC) // -20 as GWL_EXSTYLE
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExNoActivate  = 0x08000000
	wsChild         = 0x40000000

	lwaAlpha = 0x00000002

	hwndTopmost    = ^uintptr(0) // (HWND)-1
	hwndNotTopmost = ^uintptr(1) // (HWND)-2
	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010

	wmHotkey = 0x0312
	pmRemove = 0x0001

	monitorDefaultToNearest = 2
	monitorInfoPrimary      = 0x00000001
	mdtEffectiveDPI         = 0
)

// Manager owns the native window handle and any registered hotkeys.
type Manager struct {
	mu       sync.Mutex
	hwnd     windows.HWND
	baseEx   int64
	layered  bool
	through  bool
	hotkeys  map[int]func()
	hotkeyID int
	stopHot  chan struct{}
}

// New creates a platform manager. The HWND is resolved lazily because the
// native window only exists after wails.Run has started.
func New() *Manager {
	return &Manager{hotkeys: map[int]func(){}}
}

// HWND returns the process's main top-level window.
func (m *Manager) HWND() (windows.HWND, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hwnd != 0 {
		return m.hwnd, nil
	}
	hwnd, err := findMainWindow()
	if err != nil {
		return 0, err
	}
	m.hwnd = hwnd
	return hwnd, nil
}

func findMainWindow() (windows.HWND, error) {
	pid := uint32(windows.GetCurrentProcessId())
	var best windows.HWND
	var bestScore int64

	cb := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var procID uint32
		procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&procID)))
		if procID != pid {
			return 1
		}
		var rect winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
		area := int64(rect.Right-rect.Left) * int64(rect.Bottom-rect.Top)
		if area <= 0 {
			return 1
		}
		// The Wails window class is unique to the main window, so it wins even
		// while the window is still hidden. Visibility and size only break ties
		// between helper windows of the same process.
		score := area
		if windowClassName(hwnd) == "wailsWindow" {
			score += 1 << 40
		}
		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible != 0 {
			score += 1 << 30
		}
		if score > bestScore {
			bestScore = score
			best = windows.HWND(hwnd)
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	if best == 0 {
		return 0, errors.New("platform: 未找到窗口句柄")
	}
	return best, nil
}

func windowClassName(hwnd uintptr) string {
	buf := make([]uint16, 64)
	n, _, _ := procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// ---------------------------------------------------------------- geometry

type winRect struct {
	Left, Top, Right, Bottom int32
}

func (r winRect) rect() Rect {
	return Rect{X: int(r.Left), Y: int(r.Top), W: int(r.Right - r.Left), H: int(r.Bottom - r.Top)}
}

type winPoint struct {
	X, Y int32
}

type winMonitorInfo struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
}

// AbsolutePlacement reports that RestoreBounds uses absolute virtual-desktop
// coordinates, unlike the Wails position API.
func (m *Manager) AbsolutePlacement() bool { return true }

// WorkAreas lists every monitor's usable area in virtual-desktop pixels.
func (m *Manager) WorkAreas() []WorkArea {
	var areas []WorkArea
	cb := windows.NewCallback(func(hmon uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
		info := winMonitorInfo{CbSize: uint32(unsafe.Sizeof(winMonitorInfo{}))}
		ok, _, _ := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&info)))
		if ok == 0 {
			return 1
		}
		areas = append(areas, WorkArea{
			Rect:    info.RcWork.rect(),
			Primary: info.DwFlags&monitorInfoPrimary != 0,
		})
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	return areas
}

// WindowRect returns the main window's absolute rectangle in physical pixels.
// A minimised window reports a bogus (-32000) position, so that state is
// rejected instead of being persisted as the next launch position.
func (m *Manager) WindowRect() (Rect, error) {
	hwnd, err := m.HWND()
	if err != nil {
		return Rect{}, err
	}
	if iconic, _, _ := procIsIconic.Call(uintptr(hwnd)); iconic != 0 {
		return Rect{}, errors.New("platform: 窗口已最小化")
	}
	var r winRect
	if ok, _, callErr := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return Rect{}, callErr
	}
	return r.rect(), nil
}

// RestoreBounds places the window at an absolute position. x/y are physical
// virtual-desktop pixels (what WindowGetPosition reports), while w/h are DIPs
// (what WindowGetSize reports); the size is converted with the DPI of the
// monitor the window is about to land on.
//
// Wails' own WindowSetPosition cannot be used here: its Win32 backend treats
// the arguments as offsets from the current monitor's work-area origin while
// WindowGetPosition returns absolute coordinates. On a secondary monitor that
// shifts the window by the monitor origin, often far off-screen.
func (m *Manager) RestoreBounds(x, y, w, h int) (Rect, error) {
	hwnd, err := m.HWND()
	if err != nil {
		return Rect{}, err
	}
	if w <= 0 || h <= 0 {
		return Rect{}, errors.New("platform: 无效的窗口尺寸")
	}

	dpi := dpiAtPoint(x, y)
	target := Rect{
		X: x,
		Y: y,
		W: w * dpi / 96,
		H: h * dpi / 96,
	}
	final := fitToWorkAreas(target, m.WorkAreas())

	ok, _, callErr := procSetWindowPos.Call(uintptr(hwnd), 0, uintptr(final.X), uintptr(final.Y),
		uintptr(final.W), uintptr(final.H), swpNoZOrder|swpNoActivate)
	if ok == 0 {
		return Rect{}, callErr
	}
	return final, nil
}

// dpiAtPoint returns the effective DPI of the monitor nearest to a point.
func dpiAtPoint(x, y int) int {
	if err := procGetDpiForMonitor.Find(); err != nil {
		return 96
	}
	pt := winPoint{X: int32(x), Y: int32(y)}
	hmon, _, _ := procMonitorFromPoint.Call(uintptr(unsafe.Pointer(&pt)), monitorDefaultToNearest)
	if hmon == 0 {
		return 96
	}
	var dpiX, dpiY uint32
	hr, _, _ := procGetDpiForMonitor.Call(hmon, mdtEffectiveDPI, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	if int32(hr) != 0 || dpiX == 0 {
		return 96
	}
	return int(dpiX)
}

// SetOpacity applies whole-window alpha (0.2 – 1.0).
func (m *Manager) SetOpacity(alpha float64) error {
	hwnd, err := m.HWND()
	if err != nil {
		return err
	}
	if alpha >= 0.999 {
		return m.clearLayered(hwnd)
	}
	if alpha < 0.2 {
		alpha = 0.2
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.layered {
		ex, _, _ := procGetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle)
		m.baseEx = int64(ex)
		_, _, _ = procSetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle, uintptr(m.baseEx|wsExLayered))
		m.layered = true
	}
	value := byte(alpha * 255)
	ok, _, callErr := procSetLayeredWindowAttrs.Call(uintptr(hwnd), 0, uintptr(value), lwaAlpha)
	if ok == 0 {
		return callErr
	}
	return nil
}

func (m *Manager) clearLayered(hwnd windows.HWND) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.layered {
		return nil
	}
	_, _, _ = procSetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle, uintptr(m.baseEx))
	m.layered = false
	m.through = false
	return nil
}

// SetClickThrough toggles WS_EX_TRANSPARENT so clicks fall to the window below.
func (m *Manager) SetClickThrough(enabled bool) error {
	hwnd, err := m.HWND()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	ex, _, _ := procGetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle)
	if !m.layered {
		m.baseEx = int64(ex)
	}
	next := m.baseEx | wsExLayered
	if enabled {
		next |= wsExTransparent
	}
	_, _, _ = procSetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle, uintptr(next))
	m.layered = true
	m.through = enabled
	return nil
}

// ClickThrough reports the current state.
func (m *Manager) ClickThrough() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.through
}

// SetTopmost pins or unpins the window.
func (m *Manager) SetTopmost(on bool) error {
	hwnd, err := m.HWND()
	if err != nil {
		return err
	}
	insertAfter := hwndNotTopmost
	if on {
		insertAfter = hwndTopmost
	}
	ok, _, callErr := procSetWindowPos.Call(uintptr(hwnd), insertAfter, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate)
	if ok == 0 {
		return callErr
	}
	return nil
}

// Embed parents the window into another process window.
func (m *Manager) Embed(targetTitle string) error {
	hwnd, err := m.HWND()
	if err != nil {
		return err
	}
	target, err := FindWindowByTitle(targetTitle)
	if err != nil {
		return err
	}
	if ok, _, callErr := procSetParent.Call(uintptr(hwnd), uintptr(target)); ok == 0 {
		return callErr
	}
	return nil
}

// Unembed restores the desktop as the parent.
func (m *Manager) Unembed() error {
	hwnd, err := m.HWND()
	if err != nil {
		return err
	}
	if ok, _, callErr := procSetParent.Call(uintptr(hwnd), 0); ok == 0 {
		return callErr
	}
	return nil
}

// FindWindowByTitle locates a visible top-level window by a case-insensitive
// title fragment.
func FindWindowByTitle(fragment string) (windows.HWND, error) {
	var found windows.HWND
	lowered := toLowerUTF16(fragment)
	cb := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		length, _, _ := user32.NewProc("GetWindowTextLengthW").Call(hwnd)
		if length == 0 || length > 512 {
			return 1
		}
		buf := make([]uint16, length+1)
		user32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		title := toLowerUTF16(windows.UTF16ToString(buf))
		if title == lowered || contains(title, lowered) {
			found = windows.HWND(hwnd)
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	if found == 0 {
		return 0, errors.New("platform: 未找到目标窗口")
	}
	return found, nil
}

// RegisterHotkeys installs global hotkeys and invokes handlers from a private
// message loop on a locked OS thread.
func (m *Manager) RegisterHotkeys(bindings []Hotkey, handler func(name string)) error {
	m.mu.Lock()
	for id := range m.hotkeys {
		procUnregisterHotKey.Call(0, uintptr(id))
	}
	m.hotkeys = map[int]func(){}
	m.hotkeyID = 0x5744 // "WD"

	type reg struct {
		id  int
		vk  uint32
		mod uint32
		fn  func()
	}
	var regs []reg
	for _, b := range bindings {
		vk, mod, ok := parseHotkey(b.Accelerator)
		if !ok {
			continue
		}
		id := m.hotkeyID
		m.hotkeyID++
		name := b.Name
		regs = append(regs, reg{id: id, vk: vk, mod: mod, fn: func() { handler(name) }})
	}
	m.mu.Unlock()

	for _, r := range regs {
		r := r
		ok, _, _ := procRegisterHotKey.Call(0, uintptr(r.id), uintptr(r.mod), uintptr(r.vk))
		if ok == 0 {
			continue
		}
		m.mu.Lock()
		m.hotkeys[r.id] = r.fn
		m.mu.Unlock()
	}
	if len(m.hotkeys) == 0 {
		return errors.New("platform: 没有全局快捷键注册成功")
	}
	go m.hotkeyLoop()
	return nil
}

func (m *Manager) hotkeyLoop() {
	// A dedicated thread is required: WM_HOTKEY with a NULL window handle is
	// posted to the registering thread's queue, so this goroutine pins itself
	// to one OS thread for its whole life.
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		var msg struct {
			HWnd    uintptr
			Message uint32
			WParam  uintptr
			LParam  uintptr
			Time    uint32
			Pt      struct{ X, Y int32 }
		}
		for {
			ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(ret) <= 0 {
				return
			}
			if msg.Message == wmHotkey {
				m.mu.Lock()
				fn := m.hotkeys[int(msg.WParam)]
				m.mu.Unlock()
				if fn != nil {
					go fn()
				}
				continue
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

// UnregisterHotkeys removes every binding.
func (m *Manager) UnregisterHotkeys() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.hotkeys {
		procUnregisterHotKey.Call(0, uintptr(id))
	}
	m.hotkeys = map[int]func(){}
}

// Hotkey is one accelerator binding.
type Hotkey struct {
	Name        string `json:"name"`
	Accelerator string `json:"accelerator"`
}

func parseHotkey(acc string) (vk uint32, mod uint32, ok bool) {
	mod = 0x4000 // MOD_NOREPEAT
	for _, part := range splitPlus(acc) {
		switch norm(part) {
		case "ctrl", "control":
			mod |= 0x0002
		case "alt":
			mod |= 0x0001
		case "shift":
			mod |= 0x0004
		case "win", "meta":
			mod |= 0x0008
		default:
			v := virtualKey(part)
			if v == 0 {
				return 0, 0, false
			}
			vk = v
		}
	}
	return vk, mod, vk != 0
}

func norm(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		out = append(out, r)
	}
	return string(out)
}

func splitPlus(s string) []string {
	var parts []string
	cur := ""
	for _, r := range s {
		if r == '+' {
			if cur != "" {
				parts = append(parts, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

func virtualKey(name string) uint32 {
	switch norm(name) {
	case "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
		"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z":
		return uint32(name[0] & 0xdf)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return uint32(name[0])
	case "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
		idx := 0
		for i := 1; i < len(name); i++ {
			idx = idx*10 + int(name[i]-'0')
		}
		return 0x70 + uint32(idx) - 1
	case "space":
		return 0x20
	case "enter", "return":
		return 0x0D
	case "esc", "escape":
		return 0x1B
	case "tab":
		return 0x09
	case "left":
		return 0x25
	case "up":
		return 0x26
	case "right":
		return 0x27
	case "down":
		return 0x28
	case "home":
		return 0x24
	case "end":
		return 0x23
	case "pgup", "pageup":
		return 0x21
	case "pgdn", "pagedown":
		return 0x22
	}
	return 0
}

func toLowerUTF16(s string) string {
	return norm(s)
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// windowsCallback adapts a Go closure for use as a Win32 callback.
func windowsCallback(fn func(a, b, c, d uintptr) uintptr) uintptr {
	return windows.NewCallback(fn)
}
