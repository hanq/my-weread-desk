// Package config owns the user-visible settings and the application data
// directory. Settings live in a single JSON document so themes can be exported
// and imported verbatim.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Theme is the full typography and colour configuration of the reading surface.
type Theme struct {
	Name             string  `json:"name"`
	FontFamily       string  `json:"fontFamily"`
	FallbackFonts    string  `json:"fallbackFonts"`
	FontSize         int     `json:"fontSize"`
	LineHeight       float64 `json:"lineHeight"`
	LetterSpacing    float64 `json:"letterSpacing"`
	ParagraphSpacing int     `json:"paragraphSpacing"`
	TextIndent       int     `json:"textIndent"`
	TextColor        string  `json:"textColor"`
	Background       string  `json:"background"`
	Surface          string  `json:"surface"`
	SurfaceOpacity   float64 `json:"surfaceOpacity"`
	TextAlign        string  `json:"textAlign"`
	PageWidth        int     `json:"pageWidth"`
	MarginX          int     `json:"marginX"`
	MarginY          int     `json:"marginY"`
	LineNumbers      bool    `json:"lineNumbers"`
	Columns          int     `json:"columns"`
	// Pattern is a Chinese paper-pattern id rendered under the text; an empty
	// value means no pattern. PatternOpacity is the overlay strength (0.04-0.4).
	Pattern        string  `json:"pattern"`
	PatternOpacity float64 `json:"patternOpacity"`
	HeadingColor   string  `json:"headingColor"`
	AccentColor    string  `json:"accentColor"`
	SelectionColor string  `json:"selectionColor"`
	ParagraphStyle string  `json:"paragraphStyle"`
}

// WindowSettings captures everything the OS needs to restore the window.
type WindowSettings struct {
	X            int     `json:"x"`
	Y            int     `json:"y"`
	W            int     `json:"w"`
	H            int     `json:"h"`
	AlwaysOnTop  bool    `json:"alwaysOnTop"`
	Opacity      float64 `json:"opacity"`
	ClickThrough bool    `json:"clickThrough"`
	Frameless    bool    `json:"frameless"`
	Maximised    bool    `json:"maximised"`
	HideOnBlur   bool    `json:"hideOnBlur"`
	Remembered   bool    `json:"remembered"`
	Radius       int     `json:"radius"`
}

// TypingSettings drives the "typing camouflage" mode.
type TypingSettings struct {
	Enabled      bool   `json:"enabled"`
	HideUnread   bool   `json:"hideUnread"`
	FaintUnread  bool   `json:"faintUnread"`
	Sound        bool   `json:"sound"`
	CursorShape  string `json:"cursorShape"`
	SkipOnEnter  bool   `json:"skipOnEnter"`
	AdvanceOnAny bool   `json:"advanceOnAny"`
}

// ReaderSettings holds behaviour toggles that are not purely visual.
type ReaderSettings struct {
	PageTurnAnimation bool   `json:"pageTurnAnimation"`
	PageMode          string `json:"pageMode"`
	PageTurnEffect    string `json:"pageTurnEffect"`
	WheelTurn         bool   `json:"wheelTurn"`
	ClickTurn         bool   `json:"clickTurn"`
	ShowScrollbar     bool   `json:"showScrollbar"`
	SmoothScroll      bool   `json:"smoothScroll"`
	AutoHideBars      bool   `json:"autoHideBars"`
	ShowStatusBar     bool   `json:"showStatusBar"`
	ReportProgress    bool   `json:"reportProgress"`
	ReportReadTime    bool   `json:"reportReadTime"`
	ReportInterval    int    `json:"reportInterval"`
	ConfirmExit       bool   `json:"confirmExit"`
	PrefetchNext      bool   `json:"prefetchNext"`
}

// Zone is an absolute screen rectangle used by the hover-reveal watcher.
type Zone struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// DisguiseSettings controls the camouflage layer: the taskbar icon, the fake
// chat identity and the boss-key / hover-reveal window behaviour.
type DisguiseSettings struct {
	TaskbarIcon bool   `json:"taskbarIcon"`
	HideTaskbar bool   `json:"hideTaskbar"`
	ContactName string `json:"contactName"`
	// Nickname and Avatar are the IM identity of the reader. Avatar stores the
	// file name inside the data directory's avatar folder, not an absolute
	// path, so moving the profile between machines keeps working.
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	SendOnEnter bool   `json:"sendOnEnter"`
	Floating    bool   `json:"floating"`
	HoverReveal bool   `json:"hoverReveal"`
	ShowZone    Zone   `json:"showZone"`
	HideZone    Zone   `json:"hideZone"`
	Migrated    bool   `json:"migrated"`
}

// AISettings configures the retrieval assistant. The API key lives in the same
// local settings file as everything else and is never sent anywhere except the
// endpoint the user configured.
type AISettings struct {
	Enabled     bool    `json:"enabled"`
	BaseURL     string  `json:"baseUrl"`
	APIKey      string  `json:"apiKey"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"maxTokens"`
	// Consent records that the user agreed to send retrieved passages to the
	// configured provider. Book text is copyrighted, so enabling the assistant
	// without it would be a silent disclosure.
	Consent bool `json:"consent"`
}

// Settings is the whole persisted configuration.
type Settings struct {
	Version       int               `json:"version"`
	Theme         Theme             `json:"theme"`
	CustomThemes  []Theme           `json:"customThemes"`
	Skin          string            `json:"skin"`
	Zen           bool              `json:"zen"`
	Typing        TypingSettings    `json:"typing"`
	Window        WindowSettings    `json:"window"`
	Reader        ReaderSettings    `json:"reader"`
	AI            AISettings        `json:"ai"`
	Disguise      DisguiseSettings  `json:"disguise"`
	Hotkeys       map[string]string `json:"hotkeys"`
	WindowTitle   string            `json:"windowTitle"`
	HideTray      bool              `json:"hideTray"`
	CloseToTray   bool              `json:"closeToTray"`
	AcceptedTerms bool              `json:"acceptedTerms"`
	LastBookID    string            `json:"lastBookId"`
}

// legacyFallbackFonts is the pre-migration serif chain. It named fonts that
// are absent on Windows, so stored settings that still carry it are upgraded to
// DefaultTheme's chain instead of silently rendering in SimSun.
const legacyFallbackFonts = "Songti SC, SimSun, serif"

// DefaultTheme is the "paper" reading theme.
//
// Source Han Serif is not installed by default on Windows and "Songti SC" is
// macOS-only, so the fallback chain names Noto Serif SC (the same typeface
// family as Source Han Serif) before SimSun; without it the paper theme
// silently rendered Chinese in SimSun.
func DefaultTheme() Theme {
	return Theme{
		Name:             "纸白",
		FontFamily:       "Source Han Serif SC",
		FallbackFonts:    "Noto Serif SC, Songti SC, SimSun, serif",
		FontSize:         19,
		LineHeight:       1.85,
		LetterSpacing:    0.02,
		ParagraphSpacing: 12,
		TextIndent:       2,
		TextColor:        "#2b2b2b",
		Background:       "#f5f2ea",
		Surface:          "#fffdf8",
		SurfaceOpacity:   1,
		TextAlign:        "justify",
		// 0 means "use the whole measure": the paper then fills the stage and
		// only the small stage margins remain around the text.
		// 0 means automatic: one column on narrow windows, two once the text
		// area is wide enough. A fixed page width keeps the manual choice.
		PageWidth:      0,
		MarginX:        12,
		MarginY:        12,
		LineNumbers:    false,
		Columns:        0,
		Pattern:        "",
		PatternOpacity: 0.14,
		HeadingColor:   "#1d1d1d",
		AccentColor:    "#2f6fed",
		SelectionColor: "rgba(47,111,237,0.22)",
		ParagraphStyle: "indent",
	}
}

// PresetThemes returns the built-in palettes.
func PresetThemes() []Theme {
	paper := DefaultTheme()

	sepia := paper
	sepia.Name = "羊皮"
	sepia.Background = "#efe3c8"
	sepia.Surface = "#f7ecd6"
	sepia.TextColor = "#4a3b28"
	sepia.HeadingColor = "#33271a"
	sepia.AccentColor = "#9a6b2f"

	night := paper
	night.Name = "夜间"
	night.Background = "#14161a"
	night.Surface = "#1b1e24"
	night.TextColor = "#c9ced8"
	night.HeadingColor = "#e8ecf3"
	night.AccentColor = "#6d9bff"
	night.SelectionColor = "rgba(109,155,255,0.28)"

	green := paper
	green.Name = "护眼"
	green.Background = "#1f2a24"
	green.Surface = "#243029"
	green.TextColor = "#b9cdb9"
	green.HeadingColor = "#d8e8d5"
	green.AccentColor = "#7bc08a"

	clear := paper
	clear.Name = "透明"
	clear.Background = "transparent"
	clear.Surface = "rgba(255,255,255,0.06)"
	clear.SurfaceOpacity = 0.06
	clear.TextColor = "#f2f4f8"
	clear.HeadingColor = "#ffffff"
	clear.AccentColor = "#8ab4ff"
	clear.PageWidth = 0
	clear.MarginX = 12
	clear.MarginY = 12

	mono := paper
	mono.Name = "等宽"
	mono.FontFamily = "Cascadia Mono"
	mono.FallbackFonts = "Consolas, JetBrains Mono, monospace"
	mono.FontSize = 16
	mono.LineHeight = 1.6
	mono.LetterSpacing = 0
	mono.TextIndent = 0
	mono.ParagraphSpacing = 8
	mono.Background = "#1e1e1e"
	mono.Surface = "#252526"
	mono.TextColor = "#d4d4d4"
	mono.HeadingColor = "#e8e8e8"
	mono.AccentColor = "#3794ff"
	mono.PageWidth = 0
	mono.LineNumbers = true

	return []Theme{paper, sepia, night, green, clear, mono}
}

// legacyWindowRadius is the radius every platform defaulted to before the
// macOS-aligned value. Migration only rewrites this exact value.
const legacyWindowRadius = 12

// defaultWindowRadius matches the host window chrome. macOS draws a visibly
// rounded, continuous corner on every window, so the default is larger there;
// the Win32 target keeps the historical 12px so existing screenshots and window
// geometry stay familiar.
func defaultWindowRadius() int {
	if runtime.GOOS == "darwin" {
		return 16
	}
	return legacyWindowRadius
}

// Defaults returns a complete default settings document.
func Defaults() *Settings {
	return &Settings{
		Version:      5,
		Theme:        DefaultTheme(),
		CustomThemes: []Theme{},
		Skin:         "none",
		Zen:          false,
		Typing: TypingSettings{
			Enabled:      false,
			HideUnread:   true,
			FaintUnread:  false,
			Sound:        false,
			CursorShape:  "bar",
			SkipOnEnter:  true,
			AdvanceOnAny: true,
		},
		Window: WindowSettings{
			W:           1080,
			H:           760,
			Opacity:     1,
			Frameless:   false,
			Remembered:  false,
			AlwaysOnTop: false,
			Radius:      defaultWindowRadius(),
		},
		Reader: ReaderSettings{
			PageTurnAnimation: true,
			PageMode:          "page",
			PageTurnEffect:    "slide",
			WheelTurn:         true,
			ClickTurn:         true,
			ShowScrollbar:     true,
			SmoothScroll:      true,
			AutoHideBars:      true,
			ShowStatusBar:     true,
			ReportProgress:    true,
			ReportReadTime:    true,
			ReportInterval:    30,
			ConfirmExit:       false,
			PrefetchNext:      true,
		},
		AI: AISettings{
			Enabled:     false,
			Temperature: 0.3,
			MaxTokens:   1200,
		},
		Disguise: DisguiseSettings{
			TaskbarIcon: true,
			ContactName: "文件传输助手",
			Nickname:    "我",
			SendOnEnter: true,
			ShowZone:    Zone{X: 0, Y: 120, W: 8, H: 180},
			HideZone:    Zone{X: 0, Y: 420, W: 8, H: 180},
			Migrated:    true,
		},
		Hotkeys: map[string]string{
			"zen":          "Ctrl+Alt+Z",
			"toggleTop":    "Ctrl+Alt+T",
			"typing":       "Ctrl+Alt+D",
			"panic":        "Ctrl+Alt+H",
			"cycleSkin":    "Ctrl+Alt+M",
			"clickThrough": "Ctrl+Alt+C",
		},
		WindowTitle: "",
		CloseToTray: true,
	}
}

// Store persists settings to disk.
type Store struct {
	mu   sync.RWMutex
	path string
	data *Settings
}

// NewStore loads (or creates) the settings file under dataDir.
func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dataDir, "settings.json")}
	raw, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		var loaded Settings
		if err := json.Unmarshal(raw, &loaded); err != nil {
			// A corrupt file must not brick the app; keep it aside.
			_ = os.Rename(s.path, s.path+".broken")
			s.data = Defaults()
		} else {
			s.data = mergeDefaults(&loaded)
		}
	case errors.Is(err, os.ErrNotExist):
		s.data = Defaults()
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return s, nil
}

// mergeDefaults fills any zero-valued field a newer build introduced.
func mergeDefaults(in *Settings) *Settings {
	def := Defaults()
	oldVersion := in.Version
	if in.Version < def.Version {
		in.Version = def.Version
	}
	if in.Theme.FontSize == 0 {
		in.Theme = def.Theme
	}
	if in.Theme.TextColor == "" {
		in.Theme.TextColor = def.Theme.TextColor
	}
	if in.Theme.Background == "" {
		in.Theme.Background = def.Theme.Background
	}
	if in.Theme.Surface == "" {
		in.Theme.Surface = def.Theme.Surface
	}
	if in.Theme.SurfaceOpacity == 0 {
		in.Theme.SurfaceOpacity = 1
	}
	if in.Theme.LineHeight == 0 {
		in.Theme.LineHeight = def.Theme.LineHeight
	}
	if in.Theme.FallbackFonts == "" || in.Theme.FallbackFonts == legacyFallbackFonts {
		in.Theme.FallbackFonts = def.Theme.FallbackFonts
	}
	if in.Theme.PageWidth == 720 && in.Theme.MarginX == 56 && in.Theme.MarginY == 44 {
		// Legacy paper layout: a fixed 720px measure inside 56/44 margins left a
		// wide empty frame on desktop windows. Move that exact signature to the
		// edge-to-edge layout; a deliberately customised measure is left alone.
		in.Theme.PageWidth = def.Theme.PageWidth
		in.Theme.MarginX = def.Theme.MarginX
		in.Theme.MarginY = def.Theme.MarginY
	}
	if !in.Disguise.Migrated {
		// Settings written before the disguise category existed have no
		// disguise block; the zero value would silently disable the taskbar icon.
		in.Disguise = def.Disguise
	}
	if in.Disguise.Nickname == "" {
		in.Disguise.Nickname = def.Disguise.Nickname
	}
	if in.Disguise.ShowZone.W == 0 && in.Disguise.ShowZone.H == 0 {
		in.Disguise.ShowZone = def.Disguise.ShowZone
	}
	if in.Disguise.HideZone.W == 0 && in.Disguise.HideZone.H == 0 {
		in.Disguise.HideZone = def.Disguise.HideZone
	}
	if in.Theme.ParagraphStyle == "" {
		in.Theme.ParagraphStyle = def.Theme.ParagraphStyle
	}
	if in.Reader.ReportInterval <= 0 {
		in.Reader.ReportInterval = def.Reader.ReportInterval
	}
	if in.Reader.PageMode == "" {
		in.Reader.PageMode = def.Reader.PageMode
	}
	switch in.Reader.PageMode {
	case "page", "scroll":
	default:
		in.Reader.PageMode = def.Reader.PageMode
	}
	if in.Reader.PageTurnEffect == "" {
		in.Reader.PageTurnEffect = def.Reader.PageTurnEffect
	}
	switch in.Reader.PageTurnEffect {
	case "slide", "fade", "none":
	default:
		in.Reader.PageTurnEffect = def.Reader.PageTurnEffect
	}
	if oldVersion < 3 {
		// These booleans did not exist before the page-mode release. Their zero
		// value would silently turn the new controls off, so seed them once.
		in.Reader.WheelTurn = def.Reader.WheelTurn
		in.Reader.ClickTurn = def.Reader.ClickTurn
		in.Reader.ShowScrollbar = def.Reader.ShowScrollbar
		in.Reader.SmoothScroll = def.Reader.SmoothScroll
	}
	if oldVersion < 4 && in.Theme.Columns == 1 {
		// Version 3 always stored a fixed column count, and 1 was the default.
		// The new default is automatic (0), so old documents opt into the
		// wide-window two-column layout; an explicit 2 is kept.
		in.Theme.Columns = 0
	}
	if in.Theme.PatternOpacity <= 0 || in.Theme.PatternOpacity > 0.4 {
		in.Theme.PatternOpacity = def.Theme.PatternOpacity
	}
	if in.Window.W <= 200 {
		in.Window.W = def.Window.W
	}
	if in.Window.H <= 200 {
		in.Window.H = def.Window.H
	}
	if in.Window.Opacity <= 0 {
		in.Window.Opacity = 1
	}
	if oldVersion < 2 && in.Window.Radius == 0 {
		// Settings written before the radius field existed get the platform look.
		in.Window.Radius = def.Window.Radius
	}
	if oldVersion < 5 && in.Window.Radius == legacyWindowRadius {
		// Version 4 shipped a 12px radius everywhere. Only an untouched default
		// is lifted to the current platform radius; a value the user picked with
		// the slider is preserved.
		in.Window.Radius = def.Window.Radius
	}
	if in.Hotkeys == nil || len(in.Hotkeys) == 0 {
		in.Hotkeys = def.Hotkeys
	}
	if in.Skin == "" {
		in.Skin = "none"
	}
	if in.Typing.CursorShape == "" {
		in.Typing.CursorShape = "bar"
	}
	return in
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.data
}

// Path returns the settings file path.
func (s *Store) Path() string { return s.path }

// Update mutates settings under lock and persists them.
func (s *Store) Update(fn func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.data)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// DataDir resolves the per-user application directory.
func DataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "my-weread-desk"), nil
}

// Platform returns a short platform tag used in window-restore logic.
func Platform() string { return runtime.GOOS }
