package app

import (
	"os"
	"path/filepath"
)

// openAppLog opens the per-user diagnostic log. Logs previously only went to
// the WebView console, which is invisible in a packaged build; image failures
// need a file the user can inspect.
func openAppLog(dir string) *os.File {
	path := filepath.Join(dir, "app.log")
	if info, err := os.Stat(path); err == nil && info.Size() > 4<<20 {
		_ = os.Remove(path + ".1")
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil
	}
	return f
}
