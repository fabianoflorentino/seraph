package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/state"
)

type statusioSummary struct {
	Result struct {
		StatusOverall struct {
			Status     string `json:"status"`
			StatusCode int    `json:"status_code"`
		} `json:"status_overall"`
		Status []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			StatusCode int    `json:"status_code"`
		} `json:"status"`
	} `json:"result"`
}

func (c *Collector) fetchStatusIO(ctx context.Context, source catalog.Source) (state.Status, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("build status.io request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)

	start := time.Now()

	response, err := c.client.Do(request)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("request status.io status: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	latency := time.Since(start)

	if response.StatusCode != http.StatusOK {
		return state.StatusUnknown, 0, &httpError{code: response.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("read status.io status: %w", err)
	}

	if len(body) > maxBodyBytes {
		return state.StatusUnknown, 0, fmt.Errorf("status.io response is too large")
	}

	var summary statusioSummary
	if err := json.Unmarshal(body, &summary); err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("decode status.io status: %w", err)
	}

	status, err := parseStatusIO(summary, source.Component)
	if err != nil {
		return state.StatusUnknown, 0, err
	}

	return status, latency, nil
}

func parseStatusIO(summary statusioSummary, component string) (state.Status, error) {
	name := strings.TrimSpace(component)

	if name != "" {
		for _, candidate := range summary.Result.Status {
			if candidate.Name == name {
				return statusioValue(candidate.Status, candidate.StatusCode)
			}
		}

		return state.StatusUnknown, fmt.Errorf("status.io component %q not found", name)
	}

	return statusioValue(
		summary.Result.StatusOverall.Status,
		summary.Result.StatusOverall.StatusCode,
	)
}

func statusioValue(label string, code int) (state.Status, error) {
	switch code {
	case 100:
		return state.StatusOperational, nil
	case 200, 300, 400, 600:
		return state.StatusDegraded, nil
	case 500:
		return state.StatusMajor, nil
	case 0:
	default:
		return state.StatusUnknown, fmt.Errorf("unknown status.io code %d", code)
	}

	switch strings.ToLower(strings.TrimSpace(label)) {
	case "operational":
		return state.StatusOperational, nil
	case "planned maintenance", "under maintenance", "degraded performance",
		"partial service disruption", "partial outage", "security event":
		return state.StatusDegraded, nil
	case "service disruption", "major outage":
		return state.StatusMajor, nil
	case "":
		return state.StatusUnknown, fmt.Errorf("status.io status is empty")
	default:
		return state.StatusUnknown, fmt.Errorf("unknown status.io status %q", label)
	}
}
