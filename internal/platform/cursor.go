package platform

// CursorInfo is the pointer position and the primary screen size in physical
// pixels. The hover-reveal watcher compares it against the configured zones.
type CursorInfo struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
