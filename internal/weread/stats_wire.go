package weread

// StatsWire mirrors /readdata/detail. Both channels serve the same payload, but
// its shape varies by period and by client: the chart series arrives as
// readTimes (an object keyed by bucket start) or dailyReadTimes, ebooks are
// nested under "book", audiobooks under "albumInfo.albumInfo", and older
// payloads put the book fields at the top level. Decoding all variants here
// keeps the E-Ink and Web clients on one conversion instead of two that can
// drift apart.
type StatsWire struct {
	BaseTime       FlexInt            `json:"baseTime"`
	TotalReadTime  FlexInt            `json:"totalReadTime"`
	ReadDays       FlexInt            `json:"readDays"`
	DayAverage     FlexInt            `json:"dayAverageReadTime"`
	Compare        FlexFloat          `json:"compare"`
	Rank           FlexRank           `json:"rank"`
	ReadTimes      FlexSeries         `json:"readTimes"`
	DailyReadTimes FlexSeries         `json:"dailyReadTimes"`
	ReadLongest    []ReadLongestEntry `json:"readLongest"`
	PreferCategory []StatsCategory    `json:"preferCategory"`
}

// StatsCategory is one bucket of preferCategory. The service spells the name as
// categoryTitle and the duration as readingTime; categoryName and readTime are
// the older spellings kept for payloads that still use them.
type StatsCategory struct {
	CategoryTitle string    `json:"categoryTitle"`
	CategoryName  string    `json:"categoryName"`
	ReadingTime   FlexInt   `json:"readingTime"`
	ReadTime      FlexInt   `json:"readTime"`
	Ratio         FlexFloat `json:"ratio"`
}

// ReadLongestEntry is one row of readLongest. The E-Ink payload nests ebooks
// under "book" and audiobooks under "albumInfo"; the flat fields cover the
// older shape that carried bookId/title directly on the entry.
type ReadLongestEntry struct {
	BookID   FlexString `json:"bookId"`
	Title    string     `json:"title"`
	Author   string     `json:"author"`
	ReadTime FlexInt    `json:"readTime"`
	Date     FlexInt    `json:"date"`
	Book     *ReadBook  `json:"book"`
	BookInfo *ReadBook  `json:"bookInfo"`
	Album    *ReadAlbum `json:"albumInfo"`
}

// ReadBook is the ebook wrapper used by readLongest entries.
type ReadBook struct {
	BookID FlexString `json:"bookId"`
	Title  string     `json:"title"`
	Author string     `json:"author"`
}

// ReadAlbumMeta carries the fields an audiobook entry exposes. AuthorName is the
// narrator or publisher; BookAuthor is the underlying book's author.
type ReadAlbumMeta struct {
	AlbumID    FlexString `json:"albumId"`
	Name       string     `json:"name"`
	AuthorName string     `json:"authorName"`
	BookAuthor string     `json:"bookAuthor"`
}

// ReadAlbum accepts both the double-wrapped (albumInfo.albumInfo.*) and the flat
// (albumInfo.*) spellings.
type ReadAlbum struct {
	ReadAlbumMeta
	Album *ReadAlbumMeta `json:"albumInfo"`
}

// ToReadStats converts the wire payload into the model the UI renders.
func (w StatsWire) ToReadStats(mode string) *ReadStats {
	out := &ReadStats{
		Mode:       mode,
		BaseTime:   w.BaseTime.Int64(),
		TotalTime:  w.TotalReadTime.Int64(),
		ReadDays:   int(w.ReadDays.Int64()),
		DayAverage: w.DayAverage.Int64(),
		Compare:    w.Compare.Float64(),
		Rank:       int(w.Rank.Int64()),
	}

	// readTimes is present for every period; the annual period additionally
	// carries a finer dailyReadTimes breakdown, so it is only a fallback.
	series := w.ReadTimes
	if len(series) == 0 {
		series = w.DailyReadTimes
	}
	out.DailyTimes = series.Int64s()
	if out.DailyTimes == nil {
		out.DailyTimes = []int64{}
	}

	// The overall payload omits dayAverageReadTime; derive it so the card does
	// not read zero for an account with years of history.
	if out.DayAverage == 0 && out.ReadDays > 0 && out.TotalTime > 0 {
		out.DayAverage = out.TotalTime / int64(out.ReadDays)
	}

	for _, entry := range w.ReadLongest {
		if session, ok := entry.readSession(); ok {
			out.Longest = append(out.Longest, session)
		}
	}

	for _, cat := range w.PreferCategory {
		seconds := cat.ReadingTime.Int64()
		if seconds == 0 {
			seconds = cat.ReadTime.Int64()
		}
		name := cat.CategoryTitle
		if name == "" {
			name = cat.CategoryName
		}
		if name == "" && seconds == 0 {
			continue
		}
		ratio := cat.Ratio.Float64()
		if ratio == 0 && out.TotalTime > 0 {
			ratio = float64(seconds) / float64(out.TotalTime)
		}
		out.Categories = append(out.Categories, ReadCategory{Name: name, Seconds: seconds, Ratio: ratio})
	}
	return out
}

// readSession resolves the first populated wrapper so ebook, audiobook and the
// legacy flat shape all produce a title the UI can show.
func (e ReadLongestEntry) readSession() (ReadSession, bool) {
	out := ReadSession{
		BookID:  e.BookID.String(),
		Title:   e.Title,
		Author:  e.Author,
		Seconds: e.ReadTime.Int64(),
		Date:    e.Date.Int64(),
	}
	fill := func(id, title, author string) {
		if out.BookID == "" {
			out.BookID = id
		}
		if out.Title == "" {
			out.Title = title
		}
		if out.Author == "" {
			out.Author = author
		}
	}
	for _, book := range []*ReadBook{e.Book, e.BookInfo} {
		if book != nil {
			fill(book.BookID.String(), book.Title, book.Author)
		}
	}
	if e.Album != nil {
		meta := &e.Album.ReadAlbumMeta
		if e.Album.Album != nil {
			meta = e.Album.Album
		}
		author := meta.BookAuthor
		if author == "" {
			author = meta.AuthorName
		}
		fill(meta.AlbumID.String(), meta.Name, author)
	}
	if out.BookID == "" && out.Title == "" {
		return out, false
	}
	return out, true
}
