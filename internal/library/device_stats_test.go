package library

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDeviceReadTimeAccumulates(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	day := time.Now().Format("2006-01-02")
	if err := s.AddDeviceReadTime(day, 10); err != nil {
		t.Fatalf("add first: %v", err)
	}
	if err := s.AddDeviceReadTime(day, 5); err != nil {
		t.Fatalf("add second: %v", err)
	}

	total, err := s.DeviceReadTotal()
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	if total != 15 {
		t.Fatalf("total = %d, want 15", total)
	}

	days, err := s.DeviceReadTimes(7)
	if err != nil {
		t.Fatalf("days: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("days = %d, want 1", len(days))
	}
	if days[0].Date != day || days[0].Seconds != 15 {
		t.Fatalf("day = %+v, want %s/15", days[0], day)
	}
}
