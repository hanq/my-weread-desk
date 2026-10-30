package web

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseInitialState(t *testing.T) {
	page := []byte(`<!doctype html><html><head><script>
		window.__INITIAL_STATE__ = {"reader":{"bookInfo":{"bookId":"43208843","title":"持续交付2.0","author":"乔梁"},
		"psvts":"abc123","pclts":"def456","token":"tok","currentChapter":{"chapterUid":44},
		"progress":{"chapterUid":44}},"other":{"x":"a}b"}};(function(){})();
	</script></head><body></body></html>`)

	st, err := parseInitialState(page)
	if err != nil {
		t.Fatalf("parseInitialState: %v", err)
	}
	if st.BookID != "43208843" || st.Title != "持续交付2.0" {
		t.Errorf("book = %+v", st)
	}
	if st.PSVTS != "abc123" || st.PCLTS != "def456" || st.Token != "tok" {
		t.Errorf("session = %+v", st)
	}
	if st.Chapter != 44 {
		t.Errorf("chapter = %d", st.Chapter)
	}
}

func TestParseInitialStateMissing(t *testing.T) {
	if _, err := parseInitialState([]byte("<html></html>")); err == nil {
		t.Fatal("expected an error for a page without the state")
	}
}

func TestScanBalancedJSONRespectsStrings(t *testing.T) {
	in := []byte(`{"a":"} not the end","b":{"c":1}}trailing`)
	out, ok := scanBalancedJSON(in)
	if !ok {
		t.Fatal("not balanced")
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, out)
	}
	if v["a"] != "} not the end" {
		t.Errorf("value = %v", v["a"])
	}
}

func TestParseCatalogDataWrapper(t *testing.T) {
	body := []byte(`{"data":[{"bookId":"43208843","format":"epub","updated":[
		{"chapterUid":1,"chapterIdx":1,"title":"前言","wordCount":100,"level":1,"paid":0,"tar":"https://res/x.tar"},
		{"chapterUid":2,"chapterIdx":2,"title":"第一章","wordCount":900,"level":1,"paid":1}
	]}]}`)
	chapters, format, err := parseCatalog(body, "43208843")
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}
	if format != "epub" || len(chapters) != 2 {
		t.Fatalf("format=%q chapters=%+v", format, chapters)
	}
	if chapters[0].Tar != "https://res/x.tar" || chapters[0].Title != "前言" {
		t.Errorf("chapter = %+v", chapters[0])
	}
	if !chapters[1].Paid {
		t.Errorf("paid flag lost: %+v", chapters[1])
	}
}

func TestParseCatalogBareObject(t *testing.T) {
	body := []byte(`{"bookId":"1","updated":[{"chapterUid":9,"chapterIdx":1,"title":"A"}]}`)
	chapters, _, err := parseCatalog(body, "1")
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}
	if len(chapters) != 1 || chapters[0].UID != 9 {
		t.Fatalf("chapters = %+v", chapters)
	}
}

func TestParseCatalogRejectsUnknownShape(t *testing.T) {
	if _, _, err := parseCatalog([]byte(`{"foo":1}`), "1"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseTarExtractsRegularFiles(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	payloads := map[string][]byte{
		"36/epub_1_2":  []byte("\x89PNG\r\n\x1a\nxxx"),
		"36/notes.txt": []byte("hello"),
	}
	for name, data := range payloads {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()

	files, err := ParseTar(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseTar: %v", err)
	}
	if string(files["epub_1_2"]) != string(payloads["36/epub_1_2"]) {
		t.Errorf("image bytes = %q", files["epub_1_2"])
	}
	if string(files["notes.txt"]) != "hello" {
		t.Errorf("text bytes = %q", files["notes.txt"])
	}
}

func TestParseTarReadsGzipArchive(t *testing.T) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	payload := []byte("\x89PNG\r\n\x1a\nxxx")
	if err := tw.WriteHeader(&tar.Header{Name: "36/epub_9_1.png", Mode: 0o644, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	tw.Close()

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	files, err := ParseTar(gz.Bytes())
	if err != nil {
		t.Fatalf("ParseTar(gzip): %v", err)
	}
	if string(files["epub_9_1.png"]) != string(payload) {
		t.Fatalf("image bytes = %q", files["epub_9_1.png"])
	}
}

func TestParseTarReadsZipArchive(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	payload := []byte("\x89PNG\r\n\x1a\nxxx")
	w, err := zw.Create("36/epub_9_2.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	files, err := ParseTar(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseTar(zip): %v", err)
	}
	if string(files["epub_9_2.png"]) != string(payload) {
		t.Fatalf("image bytes = %q", files["epub_9_2.png"])
	}
}

func TestExtractMPBody(t *testing.T) {
	page := `<html><head><title>文章标题</title></head><body>
		<div id="js_content" class="rich_media_content">
			<p>第一段</p><p>第二段 <strong>重点</strong></p>
			<script>bad()</script>
		</div></body></html>`
	body, title := extractMPBody(page)
	if title != "文章标题" {
		t.Errorf("title = %q", title)
	}
	if !strings.Contains(body, "第一段") || !strings.Contains(body, "<strong>重点</strong>") {
		t.Errorf("body = %q", body)
	}
	if strings.Contains(body, "bad()") {
		// The extraction copies children verbatim; scripts inside the body are
		// removed later by the block parser, not here.
		t.Log("script preserved for the parser to drop")
	}
}

func TestMediaNameAndMime(t *testing.T) {
	if got := MediaName("https://res.weread.qq.com/wrepub/epub_1_2"); got != "epub_1_2" {
		t.Errorf("MediaName = %q", got)
	}
	if got := ImageExt([]byte("\xff\xd8\xff\xe0")); got != ".jpg" {
		t.Errorf("ImageExt = %q", got)
	}
	if got := ImageMime(".png"); got != "image/png" {
		t.Errorf("ImageMime = %q", got)
	}
}
