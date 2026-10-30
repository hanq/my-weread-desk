package eink

import (
	"context"
	"net/url"

	"my-weread-desk/internal/weread/httpx"
	"my-weread-desk/internal/weread/session"
)

// RawResponse is an unparsed API response.
type RawResponse struct {
	Status int
	Body   []byte
}

// ProbeRaw performs a GET against the E-Ink host and returns the body without
// decoding it.
//
// The audiobook endpoints are documented only as metadata calls: no verified
// flow for obtaining a track's audio or its transcript exists. Decoding them
// into a guessed struct would hide exactly the information that has to be
// checked, so the raw body is returned for a probe command to describe.
func (c *Client) ProbeRaw(ctx context.Context, path string, query url.Values) (RawResponse, error) {
	u := session.EinkBase() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	resp, err := c.sess.Eink().Do(ctx, httpx.Request{
		Method:  "GET",
		URL:     u,
		Referer: "https://weread.qq.com/",
	})
	if err != nil {
		return RawResponse{}, err
	}
	return RawResponse{Status: resp.Status, Body: resp.Body}, nil
}

// AlbumInfo returns the raw /album/info payload.
func (c *Client) AlbumInfo(ctx context.Context, albumID string) (RawResponse, error) {
	return c.ProbeRaw(ctx, "/album/info", url.Values{"albumId": {albumID}})
}

// AlbumTracks returns the raw /album/list payload.
func (c *Client) AlbumTracks(ctx context.Context, albumID string, synckey int) (RawResponse, error) {
	return c.ProbeRaw(ctx, "/album/list", url.Values{
		"albumId": {albumID},
		"synckey": {itoa(synckey)},
	})
}

// TrackInfo returns the raw /track/info payload.
func (c *Client) TrackInfo(ctx context.Context, trackID string) (RawResponse, error) {
	return c.ProbeRaw(ctx, "/track/info", url.Values{"trackId": {trackID}})
}

// TrackText returns the raw /track/text payload. This is the only candidate for
// turning an audiobook into retrievable text: transcribing audio locally is not
// an option because it needs cgo, which this build cannot use.
func (c *Client) TrackText(ctx context.Context, trackID string) (RawResponse, error) {
	return c.ProbeRaw(ctx, "/track/text", url.Values{"trackId": {trackID}})
}

// AudioURL returns the raw /audio/geturl payload.
func (c *Client) AudioURL(ctx context.Context, audioID, quality string) (RawResponse, error) {
	return c.ProbeRaw(ctx, "/audio/geturl", url.Values{
		"audioId": {audioID},
		"type":    {"1"},
		"quality": {quality},
	})
}

// BookAlbum returns the raw /book/album payload.
func (c *Client) BookAlbum(ctx context.Context, bookID string) (RawResponse, error) {
	return c.ProbeRaw(ctx, "/book/album", url.Values{"bookId": {bookID}})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
