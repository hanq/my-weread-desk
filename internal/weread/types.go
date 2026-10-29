package weread

// Book is a shelf entry or a search result.
type Book struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Author    string  `json:"author"`
	Cover     string  `json:"cover"`
	Category  string  `json:"category"`
	Intro     string  `json:"intro"`
	Rating    float64 `json:"rating"`
	RatingN   int     `json:"ratingCount"`
	WordCount int     `json:"wordCount"`
	Finished  bool    `json:"finished"`
	Secret    bool    `json:"secret"`
	IsMP      bool    `json:"isMp"`
	LastRead  int64   `json:"lastRead"`
	Publisher string  `json:"publisher,omitempty"`
	PublishAt string  `json:"publishAt,omitempty"`
	ISBN      string  `json:"isbn,omitempty"`
	Price     float64 `json:"price,omitempty"`
	Paid      bool    `json:"paid"`
	UpdateAt  int64   `json:"updateAt"`
}

// Chapter is one table-of-contents entry.
type Chapter struct {
	UID       int64  `json:"uid"`
	Index     int    `json:"index"`
	Title     string `json:"title"`
	Level     int    `json:"level"`
	WordCount int    `json:"wordCount"`
	Paid      bool   `json:"paid"`
	Price     int    `json:"price,omitempty"`
	Tar       string `json:"tar,omitempty"`
	Format    string `json:"format,omitempty"`
}

// Progress is the cloud reading position.
type Progress struct {
	BookID       string  `json:"bookId"`
	ChapterUID   int64   `json:"chapterUid"`
	ChapterIndex int     `json:"chapterIndex"`
	Offset       int     `json:"offset"`
	Percent      float64 `json:"percent"`
	UpdateTime   int64   `json:"updateTime"`
	IsStart      bool    `json:"isStart"`
	Finished     bool    `json:"finished"`
}

// SearchResult is one page of search hits.
type SearchResult struct {
	Total   int    `json:"total"`
	HasMore bool   `json:"hasMore"`
	SID     string `json:"sid"`
	Books   []Book `json:"books"`
	Query   string `json:"query"`
}

// Note is an underline or a personal thought.
type Note struct {
	ID          string `json:"id"`
	BookID      string `json:"bookId"`
	ChapterUID  int64  `json:"chapterUid"`
	ChapterName string `json:"chapterName"`
	Text        string `json:"text"`
	Content     string `json:"content"`
	Range       string `json:"range"`
	Color       int    `json:"color"`
	CreateTime  int64  `json:"createTime"`
	Type        int    `json:"type"`
}

// ChapterContent is the rendered chapter, already normalised into blocks.
type ChapterContent struct {
	BookID     string   `json:"bookId"`
	ChapterUID int64    `json:"chapterUid"`
	Title      string   `json:"title"`
	Format     string   `json:"format"`
	Blocks     []Block  `json:"blocks"`
	CSS        string   `json:"css,omitempty"`
	FetchedAt  int64    `json:"fetchedAt"`
	Images     []string `json:"images,omitempty"`
}

// Block is the renderer-facing content unit.
type Block struct {
	Kind  string `json:"kind"`
	Level int    `json:"level,omitempty"`
	Text  string `json:"text,omitempty"`
	Src   string `json:"src,omitempty"`
	Alt   string `json:"alt,omitempty"`
	Class string `json:"class,omitempty"`
	Href  string `json:"href,omitempty"`
}

// ReaderState is the per-session state scraped from the web reader page, plus
// the entitlement flags needed for content requests.
type ReaderState struct {
	BookID  string `json:"bookId"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	PSVTS   string `json:"psvts"`
	PCLTS   string `json:"pclts"`
	Token   string `json:"token"`
	Format  string `json:"format"`
	Chapter int64  `json:"chapterUid"`
	Fetched int64  `json:"fetchedAt"`
}

// QRCode is a login challenge.
type QRCode struct {
	UID     string `json:"uid"`
	Payload string `json:"payload"`
	Image   string `json:"image"` // data:image/jpeg;base64,... (WeChat renders it) or data:image/png;base64,...
	Expires int64  `json:"expires"`
}

// LoginResult is the outcome of a completed QR login.
type LoginResult struct {
	VID       string `json:"vid"`
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	HasAPIKey bool   `json:"hasApiKey"`
	NeedOTP   bool   `json:"needOtp"`
	Message   string `json:"message"`
	Scanned   bool   `json:"scanned"`
}

// Account is the persisted identity shown in the UI.
type Account struct {
	Alias   string `json:"alias"`
	VID     string `json:"vid"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
	HasKey  bool   `json:"hasApiKey"`
	Updated int64  `json:"updatedAt"`
}

// ReadStats mirrors the subset of /readdata/detail the UI renders.
type ReadStats struct {
	Mode       string         `json:"mode"`
	BaseTime   int64          `json:"baseTime"`
	TotalTime  int64          `json:"totalReadTime"`
	DailyTimes []int64        `json:"dailyReadTimes"`
	ReadDays   int            `json:"readDays"`
	DayAverage int64          `json:"dayAverageReadTime"`
	Compare    float64        `json:"compare"`
	Longest    []ReadSession  `json:"readLongest"`
	Categories []ReadCategory `json:"preferCategory"`
	Rank       int            `json:"rank"`
}

// ReadSession is one continuous reading stretch.
type ReadSession struct {
	BookID  string `json:"bookId"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Seconds int64  `json:"seconds"`
	Date    int64  `json:"date"`
}

// ReadCategory is an aggregate by category.
type ReadCategory struct {
	Name    string  `json:"name"`
	Seconds int64   `json:"seconds"`
	Ratio   float64 `json:"ratio"`
}

// MPSource is an official-account subscription.
type MPSource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Author string `json:"author"`
	Cover  string `json:"cover"`
	Count  int    `json:"count"`
	Secret bool   `json:"secret"`
}

// MPArticle is one article of an official account.
type MPArticle struct {
	ReviewID   string `json:"reviewId"`
	Title      string `json:"title"`
	Cover      string `json:"cover"`
	CreateTime int64  `json:"createTime"`
	Account    string `json:"account"`
}
