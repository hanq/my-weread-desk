// Package platform window geometry helpers. These are shared by every OS so
// the restore logic can validate a saved rectangle against the monitors that
// are actually connected right now.
package platform

// Rect is a screen rectangle in physical pixels, in virtual-desktop
// coordinates (the same space Win32 reports through GetWindowRect).
type Rect struct {
	X int
	Y int
	W int
	H int
}

// Right and Bottom return the exclusive edges.
func (r Rect) Right() int  { return r.X + r.W }
func (r Rect) Bottom() int { return r.Y + r.H }

// Empty reports whether the rectangle has no area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Intersect returns the overlapping part of two rectangles.
func (r Rect) Intersect(o Rect) Rect {
	x1 := maxInt(r.X, o.X)
	y1 := maxInt(r.Y, o.Y)
	x2 := minInt(r.Right(), o.Right())
	y2 := minInt(r.Bottom(), o.Bottom())
	if x2 <= x1 || y2 <= y1 {
		return Rect{}
	}
	return Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

// Area returns the rectangle's pixel area.
func (r Rect) Area() int {
	if r.Empty() {
		return 0
	}
	return r.W * r.H
}

// WorkArea is one monitor's usable area plus its identity.
type WorkArea struct {
	Rect
	Primary bool
}

// minVisibleW/minVisibleH are the smallest slice of a restored window that must
// land on a real monitor. Below this the saved position is treated as stale
// (the monitor was unplugged or rearranged) and the window is re-centred.
const (
	minVisibleW = 160
	minVisibleH = 80
)

// fitToWorkAreas returns a rectangle that is fully inside one of the given work
// areas. A saved position that still has a meaningful part on screen keeps its
// monitor; anything else (unplugged monitor, off-screen restore) is centred on
// the primary work area.
func fitToWorkAreas(r Rect, areas []WorkArea) Rect {
	if len(areas) == 0 {
		return r
	}

	best := -1
	bestVisible := 0
	for i, a := range areas {
		inter := r.Intersect(a.Rect)
		if inter.W < minVisibleW || inter.H < minVisibleH {
			continue
		}
		if visible := inter.Area(); visible > bestVisible {
			bestVisible = visible
			best = i
		}
	}

	if best >= 0 {
		return clampInto(r, areas[best].Rect)
	}

	primary := areas[0].Rect
	for _, a := range areas {
		if a.Primary {
			primary = a.Rect
			break
		}
	}
	centred := Rect{
		X: primary.X + (primary.W-r.W)/2,
		Y: primary.Y + (primary.H-r.H)/2,
		W: r.W,
		H: r.H,
	}
	return clampInto(centred, primary)
}

// clampInto shrinks r to the work area if needed and pushes it fully inside.
func clampInto(r Rect, area Rect) Rect {
	if r.W > area.W {
		r.W = area.W
	}
	if r.H > area.H {
		r.H = area.H
	}
	if r.X < area.X {
		r.X = area.X
	}
	if r.Y < area.Y {
		r.Y = area.Y
	}
	if r.Right() > area.Right() {
		r.X = area.Right() - r.W
	}
	if r.Bottom() > area.Bottom() {
		r.Y = area.Bottom() - r.H
	}
	return r
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
