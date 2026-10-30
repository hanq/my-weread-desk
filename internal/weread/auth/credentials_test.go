package auth

import "testing"

func TestStoreRepairsScientificVID(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := s.Save(&Credentials{
		VID:     "5.8328018e+07",
		Cookies: map[string]string{"wr_vid": "5.8328018e+07", "wr_skey": "token"},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	c := reopened.Get()
	if c == nil {
		t.Fatal("credentials missing")
	}
	if c.VID != "58328018" {
		t.Fatalf("vid = %q, want 58328018", c.VID)
	}
	if c.Cookies["wr_vid"] != "58328018" {
		t.Fatalf("cookie vid = %q, want 58328018", c.Cookies["wr_vid"])
	}
}
