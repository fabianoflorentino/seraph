package collector

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/state"
)

type rssFeed struct {
	Channel struct {
		Items []struct{} `xml:"item"`
	} `xml:"channel"`
}

func (c *Collector) fetchRSS(ctx context.Context, source catalog.Source) (state.Status, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("build rss request: %w", err)
	}

	request.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	request.Header.Set("User-Agent", c.userAgent)

	start := time.Now()

	response, err := c.client.Do(request)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("request rss feed: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	latency := time.Since(start)

	if response.StatusCode != http.StatusOK {
		return state.StatusUnknown, 0, &httpError{code: response.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("read rss feed: %w", err)
	}

	if len(body) > maxBodyBytes {
		return state.StatusUnknown, 0, fmt.Errorf("rss feed is too large")
	}

	status, err := parseRSS(body)
	if err != nil {
		return state.StatusUnknown, 0, err
	}

	return status, latency, nil
}

func parseRSS(body []byte) (state.Status, error) {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return state.StatusUnknown, fmt.Errorf("decode rss feed: %w", err)
	}

	if len(feed.Channel.Items) > 0 {
		return state.StatusDegraded, nil
	}

	return state.StatusOperational, nil
}
