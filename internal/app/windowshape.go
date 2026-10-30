package app

// watchWindowShape used to clip the HWND to a rounded region after every
// resize. Native regions are 1-bit masks, so their edges cannot antialias and
// the corners looked jagged. The radius is now painted by the CSS frame over a
// transparent window, which gives a smooth edge at every DPI; this routine only
// clears a region left behind by an older build.
func (a *App) watchWindowShape(stop <-chan struct{}) {
	_ = a.platform.SetWindowRadius(0)
	<-stop
}
