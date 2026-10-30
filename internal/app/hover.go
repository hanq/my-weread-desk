package app

import (
	"time"

	"my-weread-desk/internal/config"
	"my-weread-desk/internal/platform"
)

// watchHover implements the A/B hover zones: entering the show rectangle
// reveals the window, entering the hide rectangle hides it. Polling is used
// instead of a low-level mouse hook so the app never installs a global hook
// that could be flagged by other software.
func (a *App) watchHover(stop <-chan struct{}) {
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		s := a.settings.Get()
		if !s.Disguise.HoverReveal {
			last = ""
			continue
		}
		pos := a.platform.CursorPosition()
		if pos.Width <= 0 || pos.Height <= 0 {
			continue
		}
		zone := ""
		switch {
		case inZone(pos, s.Disguise.ShowZone):
			zone = "show"
		case inZone(pos, s.Disguise.HideZone):
			zone = "hide"
		}
		if zone == last {
			continue
		}
		last = zone
		switch zone {
		case "show":
			if a.isHidden() {
				a.ShowWindow()
			}
		case "hide":
			if !a.isHidden() {
				a.HideWindow()
			}
		}
	}
}

func inZone(p platform.CursorInfo, z config.Zone) bool {
	return z.W > 0 && z.H > 0 && p.X >= z.X && p.X < z.X+z.W && p.Y >= z.Y && p.Y < z.Y+z.H
}
