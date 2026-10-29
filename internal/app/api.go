// Package app wires the WeRead channels, the local cache and the window
// primitives into the method surface the frontend calls.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"my-weread-desk/internal/config"
	"my-weread-desk/internal/library"
	"my-weread-desk/internal/platform"
	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/auth"
	"my-weread-desk/internal/weread/eink"
	"my-weread-desk/internal/weread/session"
	"my-weread-desk/internal/weread/web"
)

// App is the bound object. Every exported method becomes a frontend binding.
type App struct {
	ctx context.Context

	settings *config.Store
	store    *library.Store
	session  *session.Session
	eink     *eink.Client
	web      *web.Client

	dataDir  string
	platform *platform.Manager

	mu           sync.Mutex
	login        *auth.Login
	readerStates map[string]*weread.ReaderState

	reporter   *reporter
	downloader *downloader
}

// New creates the application graph. It is called before wails.Run so that
// startup failures can be reported to the user instead of crashing silently.
// The library store is also returned so the asset server can serve cached
// chapter images without going through the bindings layer.
func New() (*App, *library.Store, error) {
	dataDir, err := config.DataDir()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, err
	}

	settings, err := config.NewStore(dataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("读取设置失败: %w", err)
	}
	store, err := library.Open(filepath.Join(dataDir, "library.db"))
	if err != nil {
		return nil, nil, fmt.Errorf("打开缓存数据库失败: %w", err)
	}
	credStore, err := auth.NewStore(filepath.Join(dataDir, "account"))
	if err != nil {
		return nil, nil, fmt.Errorf("打开凭据库失败: %w", err)
	}

	sess := session.New(credStore)
	a := &App{
		settings:     settings,
		store:        store,
		session:      sess,
		eink:         eink.New(sess),
		web:          web.New(sess),
		dataDir:      dataDir,
		platform:     platform.New(),
		readerStates: map[string]*weread.ReaderState{},
	}
	a.reporter = newReporter(a)
	a.downloader = newDownloader(a)
	return a, store, nil
}

// Startup is called by Wails once the window exists.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.reporter.start()
	a.applyWindowSettings()
	a.syncWindowBackground()
}

// syncWindowBackground makes the native window background opaque unless the
// active theme (or the opacity slider) actually needs to see through it.
// Starting opaque avoids the "everything is invisible" failure mode where a
// translucent window is composited with nothing behind it.
func (a *App) syncWindowBackground() {
	if a.ctx == nil {
		return
	}
	s := a.settings.Get()
	transparent := s.Theme.Background == "transparent" || s.Theme.Surface == "transparent" || s.Window.Opacity < 0.999
	if transparent {
		runtime.WindowSetBackgroundColour(a.ctx, 0, 0, 0, 0)
		return
	}
	runtime.WindowSetBackgroundColour(a.ctx, 18, 20, 26, 255)
}

// SetWindowBackground lets the frontend re-apply the background after a theme
// change that did not go through SaveSettings.
func (a *App) SetWindowBackground() {
	a.syncWindowBackground()
}

// FlushProgress reports any pending reading position immediately.
func (a *App) FlushProgress() {
	a.reporter.flush()
}

// Shutdown is called when the app is closing.
func (a *App) Shutdown(ctx context.Context) {
	a.reporter.flush()
	a.reporter.stop()
	a.downloader.stop()
	a.platform.UnregisterHotkeys()
	_ = a.store.Close()
}

// applyWindowSettings restores the saved geometry once at startup and then
// applies the behavioural flags. Geometry is deliberately not re-applied from
// SaveSettings/UpdateSetting: those run on every settings edit, and moving the
// window on each edit is both surprising and, with a relative-position API,
// exactly how the window used to march off-screen.
func (a *App) applyWindowSettings() {
	a.restoreWindowGeometry()
	a.applyWindowFlags()
}

// restoreWindowGeometry places the window at the remembered absolute position.
//
// The position is validated against the monitors that are connected right now:
// a saved rectangle from an unplugged or rearranged monitor is re-centred on
// the primary work area instead of leaving the window off-screen, where it
// would still show a taskbar preview but could never be brought back.
func (a *App) restoreWindowGeometry() {
	if a.ctx == nil {
		return
	}
	s := a.settings.Get()
	if s.Window.W <= 0 || s.Window.H <= 0 {
		return
	}

	if s.Window.Remembered && (s.Window.X != 0 || s.Window.Y != 0) {
		if rect, err := a.platform.RestoreBounds(s.Window.X, s.Window.Y, s.Window.W, s.Window.H); err == nil {
			a.logf("恢复窗口位置：%d,%d %dx%d", rect.X, rect.Y, rect.W, rect.H)
			return
		} else {
			a.logf("恢复窗口位置失败，改用默认尺寸：%v", err)
		}
		runtime.WindowSetSize(a.ctx, s.Window.W, s.Window.H)
		// Only platforms without the absolute placement helper may use the
		// Wails position API; on Windows it is relative to the current monitor
		// and would reintroduce the off-screen bug.
		if !a.platform.AbsolutePlacement() {
			runtime.WindowSetPosition(a.ctx, s.Window.X, s.Window.Y)
		}
		return
	}

	runtime.WindowSetSize(a.ctx, s.Window.W, s.Window.H)
}

// applyWindowFlags applies everything except geometry.
func (a *App) applyWindowFlags() {
	if a.ctx == nil {
		return
	}
	s := a.settings.Get()
	if s.Window.AlwaysOnTop {
		runtime.WindowSetAlwaysOnTop(a.ctx, true)
		_ = a.platform.SetTopmost(true)
	}
	if s.Window.Opacity > 0 && s.Window.Opacity < 1 {
		_ = a.platform.SetOpacity(s.Window.Opacity)
	}
	if s.Window.ClickThrough {
		_ = a.platform.SetClickThrough(true)
	}
	a.registerHotkeys()
}

// ---------------------------------------------------------------- settings

// GetSettings returns the whole settings document.
func (a *App) GetSettings() config.Settings {
	return a.settings.Get()
}

// SaveSettings replaces the settings document.
func (a *App) SaveSettings(next config.Settings) (config.Settings, error) {
	if next.Theme.FontSize < 10 || next.Theme.FontSize > 96 {
		next.Theme.FontSize = 19
	}
	if next.Theme.LineHeight < 1 {
		next.Theme.LineHeight = 1.6
	}
	err := a.settings.Update(func(s *config.Settings) {
		*s = next
	})
	if err != nil {
		return a.settings.Get(), err
	}
	a.applyWindowFlags()
	a.syncWindowBackground()
	return a.settings.Get(), nil
}

// UpdateSetting applies a single top-level setting by key. Used for toggles
// that must not race a full-document round trip.
func (a *App) UpdateSetting(key string, value any) (config.Settings, error) {
	err := a.settings.Update(func(s *config.Settings) {
		switch key {
		case "skin":
			if v, ok := value.(string); ok {
				s.Skin = v
			}
		case "zen":
			if v, ok := value.(bool); ok {
				s.Zen = v
			}
		case "typing":
			if v, ok := value.(bool); ok {
				s.Typing.Enabled = v
			}
		case "windowTitle":
			if v, ok := value.(string); ok {
				s.WindowTitle = v
			}
		}
	})
	return a.settings.Get(), err
}

// PresetThemes returns the built-in themes.
func (a *App) PresetThemes() []config.Theme {
	return config.PresetThemes()
}

// ResetSettings restores defaults while keeping the account.
func (a *App) ResetSettings() (config.Settings, error) {
	err := a.settings.Update(func(s *config.Settings) {
		def := config.Defaults()
		def.AcceptedTerms = s.AcceptedTerms
		*s = *def
	})
	if err != nil {
		return a.settings.Get(), err
	}
	a.applyWindowSettings()
	a.syncWindowBackground()
	return a.settings.Get(), nil
}

// AcceptTerms records the first-run disclaimer.
func (a *App) AcceptTerms() error {
	return a.settings.Update(func(s *config.Settings) { s.AcceptedTerms = true })
}

// FontFamilies lists installed font families for the picker.
func (a *App) FontFamilies() []string {
	return platform.FontFamilies()
}

// DataDir returns the cache/settings directory.
func (a *App) DataDir() string { return a.dataDir }

// OpenDataDir reveals the data directory in the OS file manager.
func (a *App) OpenDataDir() {
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(a.dataDir))
}

// Version is stamped at build time.
var Version = "0.1.0"

// AppVersion returns the build version.
func (a *App) AppVersion() string { return Version }

// ---------------------------------------------------------------- window

// WindowState is the payload the title bar renders.
type WindowState struct {
	AlwaysOnTop  bool    `json:"alwaysOnTop"`
	ClickThrough bool    `json:"clickThrough"`
	Opacity      float64 `json:"opacity"`
	Zen          bool    `json:"zen"`
	Skin         string  `json:"skin"`
	Typing       bool    `json:"typing"`
	Title        string  `json:"title"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
}

// GetWindowState reports the live window flags.
func (a *App) GetWindowState() WindowState {
	s := a.settings.Get()
	w, h := runtime.WindowGetSize(a.ctx)
	return WindowState{
		AlwaysOnTop:  s.Window.AlwaysOnTop,
		ClickThrough: a.platform.ClickThrough(),
		Opacity:      s.Window.Opacity,
		Zen:          s.Zen,
		Skin:         s.Skin,
		Typing:       s.Typing.Enabled,
		Title:        s.WindowTitle,
		Width:        w,
		Height:       h,
	}
}

// SetAlwaysOnTop toggles window pinning.
func (a *App) SetAlwaysOnTop(on bool) (WindowState, error) {
	runtime.WindowSetAlwaysOnTop(a.ctx, on)
	_ = a.platform.SetTopmost(on)
	err := a.settings.Update(func(s *config.Settings) { s.Window.AlwaysOnTop = on })
	return a.GetWindowState(), err
}

// SetOpacity applies whole-window alpha.
func (a *App) SetOpacity(alpha float64) (WindowState, error) {
	if err := a.platform.SetOpacity(alpha); err != nil {
		return a.GetWindowState(), err
	}
	err := a.settings.Update(func(s *config.Settings) { s.Window.Opacity = alpha })
	a.syncWindowBackground()
	return a.GetWindowState(), err
}

// SetClickThrough lets clicks pass to the window below.
func (a *App) SetClickThrough(on bool) (WindowState, error) {
	if err := a.platform.SetClickThrough(on); err != nil {
		return a.GetWindowState(), err
	}
	err := a.settings.Update(func(s *config.Settings) { s.Window.ClickThrough = on })
	return a.GetWindowState(), err
}

// SetWindowTitle changes the OS window title (part of the disguise).
func (a *App) SetWindowTitle(title string) {
	runtime.WindowSetTitle(a.ctx, title)
	_ = a.settings.Update(func(s *config.Settings) { s.WindowTitle = title })
}

// SetZen toggles the immersive mode and hides the chrome in one step.
func (a *App) SetZen(on bool) (WindowState, error) {
	err := a.settings.Update(func(s *config.Settings) { s.Zen = on })
	return a.GetWindowState(), err
}

// SetSkin switches the disguise skin.
func (a *App) SetSkin(skin string) (WindowState, error) {
	allowed := map[string]bool{"none": true, "word": true, "vscode": true}
	if !allowed[skin] {
		skin = "none"
	}
	err := a.settings.Update(func(s *config.Settings) { s.Skin = skin })
	return a.GetWindowState(), err
}

// SetTyping toggles the typing camouflage mode.
func (a *App) SetTyping(on bool) (WindowState, error) {
	err := a.settings.Update(func(s *config.Settings) { s.Typing.Enabled = on })
	return a.GetWindowState(), err
}

// HideWindow hides the window (panic key / tray behaviour).
func (a *App) HideWindow() {
	a.saveWindowGeometry()
	runtime.WindowHide(a.ctx)
}

// ShowWindow reveals the window.
func (a *App) ShowWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, a.settings.Get().Window.AlwaysOnTop)
}

// MinimiseWindow minimises to the taskbar.
func (a *App) MinimiseWindow() {
	a.saveWindowGeometry()
	runtime.WindowMinimise(a.ctx)
}

// CloseWindow quits the app.
func (a *App) CloseWindow() {
	a.saveWindowGeometry()
	runtime.Quit(a.ctx)
}

// MaximiseWindow toggles maximisation.
func (a *App) MaximiseWindow() {
	runtime.WindowToggleMaximise(a.ctx)
}

// saveWindowGeometry stores position and size for the next launch.
func (a *App) saveWindowGeometry() {
	if a.ctx == nil {
		return
	}
	w, h := runtime.WindowGetSize(a.ctx)
	x, y := runtime.WindowGetPosition(a.ctx)
	if rect, err := a.platform.WindowRect(); err == nil {
		// Prefer the native rectangle: it is absolute and refuses to report a
		// minimised window's bogus (-32000) position.
		x, y = rect.X, rect.Y
	} else if a.platform.AbsolutePlacement() {
		a.logf("保存窗口位置失败：%v", err)
		return
	}
	_ = a.settings.Update(func(s *config.Settings) {
		s.Window.W = w
		s.Window.H = h
		s.Window.X = x
		s.Window.Y = y
		s.Window.Remembered = true
	})
}

// EmbedWindow parents the window into another application's window.
func (a *App) EmbedWindow(titleFragment string) error {
	return a.platform.Embed(titleFragment)
}

// UnembedWindow restores a normal top-level window.
func (a *App) UnembedWindow() error { return a.platform.Unembed() }

// RegisterHotkeys installs the configured global shortcuts.
func (a *App) RegisterHotkeys() error { return a.registerHotkeys() }

func (a *App) registerHotkeys() error {
	s := a.settings.Get()
	bindings := make([]platform.Hotkey, 0, len(s.Hotkeys))
	for name, acc := range s.Hotkeys {
		if acc == "" {
			continue
		}
		bindings = append(bindings, platform.Hotkey{Name: name, Accelerator: acc})
	}
	if len(bindings) == 0 {
		return nil
	}
	return a.platform.RegisterHotkeys(bindings, func(name string) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "hotkey", name)
		}
	})
}

// Logf writes a line to the in-app log view.
func (a *App) logf(format string, args ...any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf(format, args...))
	}
}

// now is overridable in tests.
var now = time.Now
