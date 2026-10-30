package config

import "testing"

func TestMergeDefaultsMigratesLegacyPaperLayout(t *testing.T) {
	in := Defaults()
	in.Theme.PageWidth = 720
	in.Theme.MarginX = 56
	in.Theme.MarginY = 44
	got := mergeDefaults(in)
	if got.Theme.PageWidth != 0 || got.Theme.MarginX != 12 || got.Theme.MarginY != 12 {
		t.Fatalf("legacy layout not migrated: %+v", got.Theme)
	}
}

func TestMergeDefaultsKeepsCustomMeasure(t *testing.T) {
	in := Defaults()
	in.Theme.PageWidth = 640
	in.Theme.MarginX = 40
	in.Theme.MarginY = 30
	got := mergeDefaults(in)
	if got.Theme.PageWidth != 640 || got.Theme.MarginX != 40 || got.Theme.MarginY != 30 {
		t.Fatalf("custom layout overwritten: %+v", got.Theme)
	}
}

func TestMergeDefaultsMigratesWindowRadius(t *testing.T) {
	in := Defaults()
	in.Version = 1
	in.Window.Radius = 0
	got := mergeDefaults(in)
	if got.Window.Radius != 12 {
		t.Fatalf("window radius not migrated: %d", got.Window.Radius)
	}
}

func TestMergeDefaultsMigratesLegacyColumns(t *testing.T) {
	in := Defaults()
	in.Version = 3
	in.Theme.Columns = 1
	got := mergeDefaults(in)
	if got.Theme.Columns != 0 {
		t.Fatalf("legacy single column not migrated to auto: %d", got.Theme.Columns)
	}
}

func TestMergeDefaultsKeepsExplicitTwoColumns(t *testing.T) {
	in := Defaults()
	in.Version = 3
	in.Theme.Columns = 2
	got := mergeDefaults(in)
	if got.Theme.Columns != 2 {
		t.Fatalf("explicit two-column layout overwritten: %d", got.Theme.Columns)
	}
}

func TestMergeDefaultsSeedsPageMode(t *testing.T) {
	in := Defaults()
	in.Version = 2
	in.Reader.PageMode = ""
	in.Reader.PageTurnEffect = ""
	in.Reader.WheelTurn = false
	in.Reader.ClickTurn = false
	in.Reader.ShowScrollbar = false
	in.Reader.SmoothScroll = false
	got := mergeDefaults(in)
	if got.Version != 4 {
		t.Fatalf("version = %d", got.Version)
	}
	if got.Reader.PageMode != "page" || got.Reader.PageTurnEffect != "slide" {
		t.Fatalf("page mode defaults = %+v", got.Reader)
	}
	if !got.Reader.WheelTurn || !got.Reader.ClickTurn || !got.Reader.ShowScrollbar || !got.Reader.SmoothScroll {
		t.Fatalf("booleans not seeded: %+v", got.Reader)
	}
}

func TestMergeDefaultsKeepsPageModeChoice(t *testing.T) {
	in := Defaults()
	in.Version = 3
	in.Reader.PageMode = "scroll"
	in.Reader.PageTurnEffect = "fade"
	in.Reader.WheelTurn = false
	in.Reader.ClickTurn = false
	in.Reader.ShowScrollbar = false
	in.Reader.SmoothScroll = false
	got := mergeDefaults(in)
	if got.Reader.PageMode != "scroll" || got.Reader.PageTurnEffect != "fade" {
		t.Fatalf("page mode overwritten: %+v", got.Reader)
	}
	if got.Reader.WheelTurn || got.Reader.ClickTurn || got.Reader.ShowScrollbar || got.Reader.SmoothScroll {
		t.Fatalf("booleans overwritten: %+v", got.Reader)
	}
}
