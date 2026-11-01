package main

import (
	"context"
	"embed"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
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
			Middleware: assetserver.ChainMiddleware(mediaMiddleware(store), imageProxyMiddleware(application), avatarMiddleware(application)),
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
		// The rounded chrome is painted by CSS. The NSWindow backing is already
		// made transparent by syncWindowBackground; the WKWebView still paints its
		// own opaque background unless this flag is set, which hides three corners.
		Mac: &mac.Options{
			WebviewIsTransparent: true,
			About: &mac.AboutInfo{
				Title:   "MY WEREAD",
				Message: "第三方微信读书桌面客户端\n阅读 · 书架 · 统计",
			},
		},
		Menu: buildAppMenu(application),
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

// avatarMiddleware serves the IM disguise avatar from the data directory. The
// renderer never receives the absolute path, so the stored setting can move
// between machines and the file stays out of the asset bundle.
func avatarMiddleware(application *app.App) assetserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "/avatar/"
			if !strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
			name, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, prefix))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			data, mime, err := app.AvatarFile(application, name)
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

// imageProxyMiddleware fetches an original WeRead image through the Go web
// session when the chapter tar could not provide it. The renderer has no
// WeRead cookies, so it cannot load res.weread.qq.com directly.
func imageProxyMiddleware(application *app.App) assetserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/media-proxy" {
				next.ServeHTTP(w, r)
				return
			}
			q := r.URL.Query()
			uid, _ := strconv.ParseInt(q.Get("uid"), 10, 64)
			data, mime, err := app.ProxyImage(r.Context(), application, q.Get("book"), uid, q.Get("src"))
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
