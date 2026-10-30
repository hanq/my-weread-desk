package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"my-weread-desk/internal/config"
)

const maxAvatarBytes = 8 << 20

// avatarMimes is the whitelist for stored avatars. The extension is produced by
// sniffing the file content, so the map also guards the asset route against
// serving arbitrary files from the data directory.
var avatarMimes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
}

func (a *App) avatarDir() string {
	return filepath.Join(a.dataDir, "avatar")
}

// ChooseAvatar asks for an image and copies it into the data directory. The
// stored name carries a timestamp so the renderer picks up the new file without
// any cache-busting logic of its own.
func (a *App) ChooseAvatar() (config.Settings, error) {
	if a.ctx == nil {
		return a.settings.Get(), errors.New("窗口尚未就绪")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择聊天头像",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp"},
		},
	})
	if err != nil {
		return a.settings.Get(), err
	}
	if strings.TrimSpace(path) == "" {
		return a.settings.Get(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return a.settings.Get(), fmt.Errorf("读取头像失败: %w", err)
	}
	if len(data) == 0 {
		return a.settings.Get(), errors.New("头像文件为空")
	}
	if len(data) > maxAvatarBytes {
		return a.settings.Get(), errors.New("头像不能超过 8MB")
	}
	ext := imageExt(data)
	if _, ok := avatarMimes[ext]; !ok {
		return a.settings.Get(), errors.New("不支持的图片格式")
	}

	dir := a.avatarDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return a.settings.Get(), fmt.Errorf("创建头像目录失败: %w", err)
	}
	name := fmt.Sprintf("avatar_%d%s", time.Now().UnixNano(), ext)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return a.settings.Get(), fmt.Errorf("保存头像失败: %w", err)
	}
	if err := a.settings.Update(func(s *config.Settings) { s.Disguise.Avatar = name }); err != nil {
		return a.settings.Get(), err
	}
	a.removeOtherAvatars(name)
	return a.settings.Get(), nil
}

// ClearAvatar removes the stored image and falls back to the nickname initial.
func (a *App) ClearAvatar() (config.Settings, error) {
	err := a.settings.Update(func(s *config.Settings) { s.Disguise.Avatar = "" })
	if err != nil {
		return a.settings.Get(), err
	}
	a.removeOtherAvatars("")
	return a.settings.Get(), nil
}

// removeOtherAvatars keeps at most the active image on disk. Errors are ignored:
// a stale file is harmless and the settings already point at the right one.
func (a *App) removeOtherAvatars(keep string) {
	entries, err := os.ReadDir(a.avatarDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == keep {
			continue
		}
		_ = os.Remove(filepath.Join(a.avatarDir(), entry.Name()))
	}
}

// AvatarFile is the asset-server entry point. It only returns files from the
// avatar folder with a whitelisted image extension; the request path never
// reaches the filesystem unchecked.
func AvatarFile(application *App, name string) ([]byte, string, error) {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return nil, "", errors.New("非法的头像名称")
	}
	ext := strings.ToLower(filepath.Ext(name))
	mime, ok := avatarMimes[ext]
	if !ok {
		return nil, "", errors.New("非法的头像格式")
	}
	data, err := os.ReadFile(filepath.Join(application.avatarDir(), name))
	if err != nil {
		return nil, "", err
	}
	return data, mime, nil
}
