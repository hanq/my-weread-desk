package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/httpx"
)

// statsWire is the shared /readdata/detail payload. /readdata/detail is
// documented as a Web/Mobile endpoint, and the browser session serves the same
// fields as the E-Ink channel.
type statsWire = weread.StatsWire

// Stats returns reading statistics through the cookie-authenticated channel.
// This is the fallback for credentials minted by the old browser login, which
// have no usable E-Ink token.
func (c *Client) Stats(ctx context.Context, mode string, baseTime int64) (*weread.ReadStats, error) {
	if mode == "" {
		mode = "monthly"
	}
	q := url.Values{}
	q.Set("mode", mode)
	q.Set("baseTime", strconv.FormatInt(baseTime, 10))

	var raw statsWire
	err := c.withRetry(ctx, func() error {
		resp, err := c.get(ctx, "/readdata/detail", q)
		if err != nil {
			return err
		}
		if resp.Status != 200 {
			return &weread.APIError{Endpoint: "readdata/detail", Status: resp.Status}
		}
		if err := httpx.CheckEnvelope("readdata/detail", resp.Status, resp.Body); err != nil {
			return err
		}
		return decodeStats(resp.Body, &raw)
	})
	if err != nil {
		return nil, err
	}

	return raw.ToReadStats(mode), nil
}

// decodeStats accepts both the bare stats object and the common
// {"succ":1,"data":{...}} envelope used by Web endpoints.
func decodeStats(body []byte, out *statsWire) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return fmt.Errorf("%w: 统计响应不是 JSON", weread.ErrProtocolChanged)
	}
	if hasStatsFields(probe) {
		return json.Unmarshal(body, out)
	}
	if data, ok := probe["data"]; ok {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%w: %v", weread.ErrProtocolChanged, err)
		}
		return nil
	}
	return fmt.Errorf("%w: 统计响应缺少字段", weread.ErrProtocolChanged)
}

func hasStatsFields(m map[string]json.RawMessage) bool {
	for _, key := range []string{"totalReadTime", "readDays", "readTimes", "dailyReadTimes", "baseTime"} {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}
