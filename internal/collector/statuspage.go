package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/state"
)

const maxBodyBytes = 2 << 20

type httpError struct {
	code int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("unexpected status %d", e.code)
}

type statuspageSummary struct {
	Status struct {
		Indicator   string `json:"indicator"`
		Description string `json:"description"`
	} `json:"status"`
	Components []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"components"`
}

func (c *Collector) fetchStatuspage(ctx context.Context, source catalog.Source) (state.Status, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("build status page request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)

	start := time.Now()

	response, err := c.client.Do(request)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("request status page: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	latency := time.Since(start)

	if response.StatusCode != http.StatusOK {
		return state.StatusUnknown, 0, &httpError{code: response.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("read status page: %w", err)
	}

	if len(body) > maxBodyBytes {
		return state.StatusUnknown, 0, errors.New("status page response is too large")
	}

	var summary statuspageSummary
	if err := json.Unmarshal(body, &summary); err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("decode status page: %w", err)
	}

	status, err := parseStatuspage(summary, source.Component)
	if err != nil {
		return state.StatusUnknown, 0, err
	}

	return status, latency, nil
}

func parseStatuspage(summary statuspageSummary, component string) (state.Status, error) {
	if name := strings.TrimSpace(component); name != "" {
		for _, candidate := range summary.Components {
			if candidate.Name == name {
				return componentStatus(candidate.Status)
			}
		}

		return state.StatusUnknown, fmt.Errorf("component %q not found", name)
	}

	indicator := strings.ToLower(strings.TrimSpace(summary.Status.Indicator))

	switch indicator {
	case "none":
		return state.StatusOperational, nil
	case "minor":
		return state.StatusDegraded, nil
	case "major", "critical":
		return state.StatusMajor, nil
	case "maintenance":
		return state.StatusDegraded, nil
	case "":
		return state.StatusUnknown, errors.New("status page has no status indicator")
	default:
		return state.StatusUnknown, fmt.Errorf("unknown status indicator %q", indicator)
	}
}

func componentStatus(raw string) (state.Status, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "operational":
		return state.StatusOperational, nil
	case "degraded_performance", "partial_outage", "under_maintenance":
		return state.StatusDegraded, nil
	case "major_outage":
		return state.StatusMajor, nil
	case "":
		return state.StatusUnknown, errors.New("component has no status")
	default:
		return state.StatusUnknown, fmt.Errorf("unknown component status %q", raw)
	}
}
