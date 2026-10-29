package main

import (
	"context"
	"embed"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"my-weread-desk/internal/app"
	"my-weread-desk/internal/config"
	"my-weread-desk/internal/library"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application, store, err := app.New()
	if err != nil {
		panic(err)
	}
	settings := application.GetSettings()

	winOpts := &options.App{
		Title:     displayTitle(settings),
		Width:     clampInt(settings.Window.W, 720, 3840),
		Height:    clampInt(settings.Window.H, 480, 2160),
		MinWidth:  560,
		MinHeight: 380,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: mediaMiddleware(store),
		},
		// Opaque by default: the transparent background is switched on at runtime
		// only when the theme or the opacity slider requires it.
		BackgroundColour: &options.RGBA{R: 18, G: 20, B: 26, A: 255},
		OnStartup:        application.Startup,
		OnShutdown:       application.Shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			application.FlushProgress()
			return false
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableFramelessWindowDecorations: true,
			Theme:                             windows.SystemDefault,
			BackdropType:                      windows.Auto,
			WebviewUserDataPath:               webviewDataPath(),
			ZoomFactor:                        1.0,
		},
		Bind: []interface{}{application},
	}

	if err := wails.Run(winOpts); err != nil {
		println("Error:", err.Error())
	}
}

func displayTitle(s config.Settings) string {
	if strings.TrimSpace(s.WindowTitle) != "" {
		return s.WindowTitle
	}
	return "my-weread-desk"
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func webviewDataPath() string {
	dir, err := config.DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "webview")
}

// mediaMiddleware serves cached chapter images straight from the library.
// The renderer must never issue an authenticated CDN request for an image.
func mediaMiddleware(store *library.Store) assetserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "/media/"
			if !strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
			rest := strings.TrimPrefix(r.URL.Path, prefix)
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) != 2 {
				http.NotFound(w, r)
				return
			}
			bookID, err := url.PathUnescape(parts[0])
			if err != nil {
				http.NotFound(w, r)
				return
			}
			name, err := url.PathUnescape(parts[1])
			if err != nil {
				http.NotFound(w, r)
				return
			}
			data, mime, err := store.Media(bookID, name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", mime)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			_, _ = w.Write(data)
		})
	}
}
