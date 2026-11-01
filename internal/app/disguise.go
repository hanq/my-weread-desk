package app

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"my-weread-desk/internal/config"
)

// The disguise icons ship as Windows .ico files and macOS-friendly .png files.
// Win32 loads icons from a path with LoadImage, while AppKit builds an NSImage
// from the file, and NSImage cannot decode .ico.
//
//go:embed icons/*
var disguiseIcons embed.FS

// disguiseIconExt picks the icon container the current platform can decode.
func disguiseIconExt() string {
	if goruntime.GOOS == "darwin" {
		return ".png"
	}
	return ".ico"
}

var skinTitles = map[string]string{
	"none":     "my-weread-desk",
	"word":     "文档1 - Word",
	"vscode":   "main.ts - Visual Studio Code",
	"wechat":   "微信",
	"dingtalk": "钉钉",
	"feishu":   "飞书",
	"wecom":    "企业微信",
}

var skinIcons = map[string]string{
	"none":     "app",
	"word":     "word",
	"vscode":   "vscode",
	"wechat":   "wechat",
	"dingtalk": "dingtalk",
	"feishu":   "feishu",
	"wecom":    "wecom",
}

// legacyTitles maps built-in window titles to the skin that owns them. A stored
// title belonging to another skin is rewritten when the skin changes; a custom
// title is kept as-is.
var legacyTitles = map[string]string{
	"my-weread-desk":               "none",
	"Document1 - Word":             "word",
	"文档1 - Word":                   "word",
	"main.ts - Visual Studio Code": "vscode",
	"微信":                           "wechat",
	"钉钉":                           "dingtalk",
	"飞书":                           "feishu",
	"企业微信":                         "wecom",
}

func resolveWindowTitle(current, skin string) string {
	if current == "" {
		return skinTitles[skin]
	}
	if owner, ok := legacyTitles[current]; ok && owner != skin {
		return skinTitles[skin]
	}
	return current
}

// syncWindowTitle rewrites a stale built-in title and pushes it to the native
// window so the taskbar and Alt-Tab preview match the active skin. A custom
// title is preserved.
func (a *App) syncWindowTitle() {
	s := a.settings.Get()
	title := resolveWindowTitle(s.WindowTitle, s.Skin)
	if title != s.WindowTitle {
		_ = a.settings.Update(func(c *config.Settings) { c.WindowTitle = title })
	}
	if a.ctx != nil {
		runtime.WindowSetTitle(a.ctx, title)
	}
}

// applyDisguise syncs the native taskbar / dock and Alt-Tab / app-switcher icon
// with the active skin. The icon is embedded in the binary and materialised
// under the data directory because the native APIs load icons from a file path.
func (a *App) applyDisguise() {
	s := a.settings.Get()
	name := "app"
	if s.Disguise.TaskbarIcon {
		if mapped, ok := skinIcons[s.Skin]; ok {
			name = mapped
		}
	}
	ext := disguiseIconExt()
	data, err := disguiseIcons.ReadFile("icons/" + name + ext)
	if err != nil {
		return
	}
	dir := filepath.Join(a.dataDir, "icons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, name+ext)
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, data) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return
		}
	}
	if err := a.platform.SetWindowIcon(path); err != nil {
		a.logf("设置任务栏图标失败：%v", err)
	}
	a.applyTaskbarHidden(s.Disguise.HideTaskbar)
}
