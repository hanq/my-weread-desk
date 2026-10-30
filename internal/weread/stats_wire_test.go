package weread

import (
	"encoding/json"
	"testing"
)

func TestStatsWireReadTimesObjectOrder(t *testing.T) {
	body := []byte(`{"baseTime":100,"totalReadTime":60,"readDays":2,
		"readTimes":{"300":30,"100":10,"200":20}}`)
	var wire StatsWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := wire.ToReadStats("weekly")
	want := []int64{10, 20, 30}
	if len(got.DailyTimes) != len(want) {
		t.Fatalf("dailyTimes = %v", got.DailyTimes)
	}
	for i := range want {
		if got.DailyTimes[i] != want[i] {
			t.Fatalf("dailyTimes = %v, want %v", got.DailyTimes, want)
		}
	}
	wantAt := []int64{100, 200, 300}
	if len(got.DailyTimesAt) != len(wantAt) {
		t.Fatalf("dailyTimesAt = %v", got.DailyTimesAt)
	}
	for i := range wantAt {
		if got.DailyTimesAt[i] != wantAt[i] {
			t.Fatalf("dailyTimesAt = %v, want %v", got.DailyTimesAt, wantAt)
		}
	}
}

// The annual payload spells the series as an object. A strict []FlexInt field
// rejected the whole payload with "cannot unmarshal object", which surfaced as
// the protocol-changed error in the UI.
func TestStatsWireDailyReadTimesObject(t *testing.T) {
	body := []byte(`{"totalReadTime":90,"readDays":3,
		"dailyReadTimes":{"200":20,"100":10}}`)
	var wire StatsWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := wire.ToReadStats("annually")
	if len(got.DailyTimes) != 2 || got.DailyTimes[0] != 10 || got.DailyTimes[1] != 20 {
		t.Fatalf("dailyTimes = %v", got.DailyTimes)
	}
	if len(got.DailyTimesAt) != 2 || got.DailyTimesAt[0] != 100 || got.DailyTimesAt[1] != 200 {
		t.Fatalf("dailyTimesAt = %v", got.DailyTimesAt)
	}
}

func TestStatsWireReadLongestShapes(t *testing.T) {
	body := []byte(`{"totalReadTime":100,"readDays":1,"readLongest":[
		{"book":{"bookId":"b1","title":"书一","author":"作者一"},"readTime":50},
		{"albumInfo":{"albumInfo":{"albumId":"a1","name":"听书一","authorName":"主播","bookAuthor":"原作者"}},"readTime":30},
		{"bookId":"b2","title":"书二","author":"作者二","readTime":20}
	]}`)
	var wire StatsWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := wire.ToReadStats("weekly")
	if len(got.Longest) != 3 {
		t.Fatalf("longest = %+v", got.Longest)
	}
	if got.Longest[0].BookID != "b1" || got.Longest[0].Title != "书一" || got.Longest[0].Author != "作者一" {
		t.Fatalf("ebook = %+v", got.Longest[0])
	}
	// Audiobooks prefer the underlying book author over the narrator.
	if got.Longest[1].BookID != "a1" || got.Longest[1].Title != "听书一" || got.Longest[1].Author != "原作者" {
		t.Fatalf("album = %+v", got.Longest[1])
	}
	if got.Longest[2].BookID != "b2" || got.Longest[2].Title != "书二" {
		t.Fatalf("flat = %+v", got.Longest[2])
	}
	if got.Longest[0].Seconds != 50 || got.Longest[1].Seconds != 30 {
		t.Fatalf("seconds = %+v", got.Longest)
	}
}

func TestStatsWirePreferCategory(t *testing.T) {
	body := []byte(`{"totalReadTime":1000,"readDays":1,
		"preferCategory":[{"categoryTitle":"历史","readingTime":250,"val":25}]}`)
	var wire StatsWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cats := wire.ToReadStats("monthly").Categories
	if len(cats) != 1 {
		t.Fatalf("categories = %+v", cats)
	}
	if cats[0].Name != "历史" || cats[0].Seconds != 250 || cats[0].Ratio != 0.25 {
		t.Fatalf("category = %+v", cats[0])
	}
}

func TestStatsWireDerivesDayAverage(t *testing.T) {
	// The overall payload has no dayAverageReadTime field.
	body := []byte(`{"totalReadTime":900,"readDays":3}`)
	var wire StatsWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := wire.ToReadStats("overall").DayAverage; got != 300 {
		t.Fatalf("dayAverage = %d", got)
	}
}

func TestFlexSeriesArrayAndNull(t *testing.T) {
	var series FlexSeries
	if err := json.Unmarshal([]byte(`[3,4]`), &series); err != nil {
		t.Fatalf("array: %v", err)
	}
	if got := series.Int64s(); len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Fatalf("int64s = %v", got)
	}
	if series.Times != nil {
		t.Fatalf("array must not carry times: %v", series.Times)
	}
	if err := json.Unmarshal([]byte(`null`), &series); err != nil || !series.Empty() {
		t.Fatalf("null: %+v %v", series, err)
	}
}

func TestFlexSeriesObjectKeepsTimes(t *testing.T) {
	var series FlexSeries
	if err := json.Unmarshal([]byte(`{"200":20,"100":10}`), &series); err != nil {
		t.Fatalf("object: %v", err)
	}
	if series.Len() != 2 || series.Times[0] != 100 || series.Times[1] != 200 {
		t.Fatalf("series = %+v", series)
	}
}
