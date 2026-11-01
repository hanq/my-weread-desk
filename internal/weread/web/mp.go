package web

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/httpx"
)

// mpArticlesWire mirrors GET /web/mp/articles. The browser and E-Ink endpoints
// share this review/subReview shape; only the path and authentication differ.
type mpArticlesWire struct {
	Reviews []struct {
		SubReviews []struct {
			Review struct {
				ReviewID   string         `json:"reviewId"`
				CreateTime weread.FlexInt `json:"createTime"`
				MPInfo     struct {
					Title  string `json:"title"`
					PicURL string `json:"pic_url"`
				} `json:"mpInfo"`
			} `json:"review"`
		} `json:"subReviews"`
	} `json:"reviews"`
}

// MPArticles lists an official account's articles through the cookie session.
// It is used when the E-Ink MP endpoint is unavailable or returns an empty page.
func (c *Client) MPArticles(ctx context.Context, accountID string, offset int) ([]weread.MPArticle, error) {
	q := url.Values{}
	q.Set("bookId", accountID)
	q.Set("offset", strconv.Itoa(offset))

	var raw mpArticlesWire
	err := c.withRetry(ctx, func() error {
		resp, err := c.get(ctx, "/web/mp/articles", q)
		if err != nil {
			return err
		}
		if err := httpx.CheckEnvelope("web/mp/articles", resp.Status, resp.Body); err != nil {
			return err
		}
		return json.Unmarshal(resp.Body, &raw)
	})
	if err != nil {
		return nil, err
	}

	// Keep a non-nil slice so the renderer receives [] rather than null when an
	// account has no articles.
	out := make([]weread.MPArticle, 0, 16)
	for _, group := range raw.Reviews {
		for _, sub := range group.SubReviews {
			review := sub.Review
			if review.ReviewID == "" {
				continue
			}
			out = append(out, weread.MPArticle{
				ReviewID:   review.ReviewID,
				Title:      review.MPInfo.Title,
				Cover:      review.MPInfo.PicURL,
				CreateTime: review.CreateTime.Int64(),
				Account:    accountID,
			})
		}
	}
	return out, nil
}
