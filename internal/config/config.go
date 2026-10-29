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
	HeadingColor     string  `json:"headingColor"`
	AccentColor      string  `json:"accentColor"`
	SelectionColor   string  `json:"selectionColor"`
	ParagraphStyle   string  `json:"paragraphStyle"`
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
	PageTurnAnimation bool `json:"pageTurnAnimation"`
	AutoHideBars      bool `json:"autoHideBars"`
	ShowStatusBar     bool `json:"showStatusBar"`
	ReportProgress    bool `json:"reportProgress"`
	ReportReadTime    bool `json:"reportReadTime"`
	ReportInterval    int  `json:"reportInterval"`
	ConfirmExit       bool `json:"confirmExit"`
	PrefetchNext      bool `json:"prefetchNext"`
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
	Hotkeys       map[string]string `json:"hotkeys"`
	WindowTitle   string            `json:"windowTitle"`
	HideTray      bool              `json:"hideTray"`
	CloseToTray   bool              `json:"closeToTray"`
	AcceptedTerms bool              `json:"acceptedTerms"`
	LastBookID    string            `json:"lastBookId"`
}

// DefaultTheme is the "paper" reading theme.
func DefaultTheme() Theme {
	return Theme{
		Name:             "纸白",
		FontFamily:       "Source Han Serif SC",
		FallbackFonts:    "Songti SC, SimSun, serif",
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
		PageWidth:        720,
		MarginX:          56,
		MarginY:          44,
		LineNumbers:      false,
		Columns:          1,
		HeadingColor:     "#1d1d1d",
		AccentColor:      "#2f6fed",
		SelectionColor:   "rgba(47,111,237,0.22)",
		ParagraphStyle:   "indent",
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
	clear.MarginX = 28
	clear.MarginY = 20

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

// Defaults returns a complete default settings document.
func Defaults() *Settings {
	return &Settings{
		Version:      1,
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
		},
		Reader: ReaderSettings{
			PageTurnAnimation: true,
			AutoHideBars:      true,
			ShowStatusBar:     true,
			ReportProgress:    true,
			ReportReadTime:    true,
			ReportInterval:    30,
			ConfirmExit:       false,
			PrefetchNext:      true,
		},
		Hotkeys: map[string]string{
			"zen":          "Ctrl+Alt+Z",
			"toggleTop":    "Ctrl+Alt+T",
			"typing":       "Ctrl+Alt+D",
			"panic":        "Ctrl+Alt+H",
			"cycleSkin":    "Ctrl+Alt+M",
			"clickThrough": "Ctrl+Alt+C",
		},
		WindowTitle: "Document1 - Word",
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
	if in.Version == 0 {
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
	if in.Theme.ParagraphStyle == "" {
		in.Theme.ParagraphStyle = def.Theme.ParagraphStyle
	}
	if in.Reader.ReportInterval <= 0 {
		in.Reader.ReportInterval = def.Reader.ReportInterval
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
