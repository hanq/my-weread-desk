package web

import (
	"errors"
	"testing"

	"my-weread-desk/internal/weread"
)

func TestDecodeStatsDirect(t *testing.T) {
	var out statsWire
	body := []byte(`{"baseTime":1760000000,"totalReadTime":1234,"readDays":5,"dailyReadTimes":[10,20]}`)
	if err := decodeStats(body, &out); err != nil {
		t.Fatalf("decodeStats: %v", err)
	}
	if out.TotalReadTime.Int64() != 1234 || out.ReadDays.Int64() != 5 || out.DailyReadTimes.Len() != 2 {
		t.Fatalf("decoded = %+v", out)
	}
}

// The annual payload spells dailyReadTimes as an object; decoding must not
// reject it as a type error.
func TestDecodeStatsObjectSeries(t *testing.T) {
	var out statsWire
	body := []byte(`{"baseTime":1,"totalReadTime":60,"readDays":2,"dailyReadTimes":{"200":20,"100":10}}`)
	if err := decodeStats(body, &out); err != nil {
		t.Fatalf("decodeStats: %v", err)
	}
	got := out.ToReadStats("annually")
	if len(got.DailyTimes) != 2 || got.DailyTimes[0] != 10 || got.DailyTimes[1] != 20 {
		t.Fatalf("dailyTimes = %v", got.DailyTimes)
	}
}

func TestDecodeStatsEnvelope(t *testing.T) {
	var out statsWire
	body := []byte(`{"succ":1,"data":{"baseTime":1760000000,"totalReadTime":4321}}`)
	if err := decodeStats(body, &out); err != nil {
		t.Fatalf("decodeStats: %v", err)
	}
	if out.TotalReadTime.Int64() != 4321 {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestDecodeStatsRejectsUnknownShape(t *testing.T) {
	var out statsWire
	err := decodeStats([]byte(`{"foo":1}`), &out)
	if !errors.Is(err, weread.ErrProtocolChanged) {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeStatsRankObject(t *testing.T) {
	var out statsWire
	body := []byte(`{"baseTime":1,"totalReadTime":2,"rank":{"rank":12,"total":100}}`)
	if err := decodeStats(body, &out); err != nil {
		t.Fatalf("decodeStats: %v", err)
	}
	if out.Rank.Int64() != 12 {
		t.Fatalf("rank = %d", out.Rank.Int64())
	}
}

func TestDecodeStatsRankNested(t *testing.T) {
	var out statsWire
	body := []byte(`{"baseTime":1,"totalReadTime":2,"rank":{"value":"第 9 名"}}`)
	if err := decodeStats(body, &out); err != nil {
		t.Fatalf("decodeStats: %v", err)
	}
	if out.Rank.Int64() != 9 {
		t.Fatalf("rank = %d", out.Rank.Int64())
	}
}

func TestDecodeStatsRankUnknownObject(t *testing.T) {
	var out statsWire
	body := []byte(`{"baseTime":1,"totalReadTime":2,"rank":{"foo":1}}`)
	if err := decodeStats(body, &out); err != nil {
		t.Fatalf("decodeStats: %v", err)
	}
	if out.Rank.Int64() != 0 {
		t.Fatalf("rank = %d", out.Rank.Int64())
	}
}
