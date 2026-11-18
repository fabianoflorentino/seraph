package collector

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/state"
)

type slackStatus struct {
	Status          string            `json:"status"`
	ActiveIncidents []json.RawMessage `json:"active_incidents"`
}

type stripeStatus struct {
	Largestatus string            `json:"largestatus"`
	Statuses    map[string]string `json:"statuses"`
}

func (c *Collector) fetchJSON(ctx context.Context, source catalog.Source) (state.Status, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("build json status request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)

	start := time.Now()

	response, err := c.client.Do(request)
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("request json status: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	latency := time.Since(start)

	if response.StatusCode != http.StatusOK {
		return state.StatusUnknown, 0, &httpError{code: response.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return state.StatusUnknown, 0, fmt.Errorf("read json status: %w", err)
	}

	if len(body) > maxBodyBytes {
		return state.StatusUnknown, 0, fmt.Errorf("json status response is too large")
	}

	status, err := parseJSONStatus(source.Format, body, c.now())
	if err != nil {
		return state.StatusUnknown, 0, err
	}

	return status, latency, nil
}

func parseJSONStatus(format string, body []byte, now time.Time) (state.Status, error) {
	switch format {
	case catalog.JSONFormatSlack:
		return parseSlackStatus(body)
	case catalog.JSONFormatStripe:
		return parseStripeStatus(body)
	case catalog.JSONFormatAWS:
		return parseAWSStatus(body)
	case catalog.JSONFormatGCP:
		return parseGCPStatus(body, now)
	default:
		return state.StatusUnknown, fmt.Errorf("unsupported json format %q", format)
	}
}

func parseSlackStatus(body []byte) (state.Status, error) {
	var payload slackStatus
	if err := json.Unmarshal(body, &payload); err != nil {
		return state.StatusUnknown, fmt.Errorf("decode slack status: %w", err)
	}

	switch status := strings.ToLower(strings.TrimSpace(payload.Status)); status {
	case "ok":
		if len(payload.ActiveIncidents) > 0 {
			return state.StatusDegraded, nil
		}

		return state.StatusOperational, nil
	case "notice", "degraded", "degraded_performance", "partial_outage", "maintenance":
		return state.StatusDegraded, nil
	case "critical", "major", "outage", "down":
		return state.StatusMajor, nil
	case "":
		return state.StatusUnknown, fmt.Errorf("slack status is empty")
	default:
		return state.StatusUnknown, fmt.Errorf("unknown slack status %q", status)
	}
}

func parseStripeStatus(body []byte) (state.Status, error) {
	var payload stripeStatus
	if err := json.Unmarshal(body, &payload); err != nil {
		return state.StatusUnknown, fmt.Errorf("decode stripe status: %w", err)
	}

	if status := strings.ToLower(strings.TrimSpace(payload.Largestatus)); status != "" {
		switch status {
		case "up":
			return state.StatusOperational, nil
		case "degraded":
			return state.StatusDegraded, nil
		case "down", "outage":
			return state.StatusMajor, nil
		default:
			return state.StatusUnknown, fmt.Errorf("unknown stripe status %q", status)
		}
	}

	if len(payload.Statuses) == 0 {
		return state.StatusUnknown, fmt.Errorf("stripe status is empty")
	}

	worst := state.StatusOperational

	for name, raw := range payload.Statuses {
		var current state.Status

		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "up":
			current = state.StatusOperational
		case "degraded", "maintenance":
			current = state.StatusDegraded
		case "down", "outage":
			current = state.StatusMajor
		default:
			return state.StatusUnknown, fmt.Errorf("unknown stripe status %q for %s", raw, name)
		}

		if statusRank(current) > statusRank(worst) {
			worst = current
		}
	}

	return worst, nil
}

func parseAWSStatus(body []byte) (state.Status, error) {
	decoded, err := decodeUTF16(body)
	if err != nil {
		return state.StatusUnknown, fmt.Errorf("decode aws status encoding: %w", err)
	}

	var events []struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(decoded, &events); err != nil {
		return state.StatusUnknown, fmt.Errorf("decode aws status: %w", err)
	}

	for _, event := range events {
		switch strings.TrimSpace(event.Status) {
		case "", "3":
			continue
		case "1", "2":
			return state.StatusDegraded, nil
		default:
			return state.StatusUnknown, fmt.Errorf("unknown aws event status %q", event.Status)
		}
	}

	return state.StatusOperational, nil
}

func parseGCPStatus(body []byte, now time.Time) (state.Status, error) {
	var incidents []struct {
		Begin string `json:"begin"`
		End   string `json:"end"`
	}

	if err := json.Unmarshal(body, &incidents); err != nil {
		return state.StatusUnknown, fmt.Errorf("decode gcp status: %w", err)
	}

	for _, incident := range incidents {
		if strings.TrimSpace(incident.End) == "" {
			return state.StatusDegraded, nil
		}

		end, err := time.Parse(time.RFC3339, incident.End)
		if err != nil {
			return state.StatusUnknown, fmt.Errorf("parse gcp incident end: %w", err)
		}

		if end.After(now) {
			return state.StatusDegraded, nil
		}
	}

	return state.StatusOperational, nil
}

func decodeUTF16(data []byte) ([]byte, error) {
	if len(data) < 2 {
		return data, nil
	}

	var order binary.ByteOrder = binary.LittleEndian

	start := 0

	switch {
	case data[0] == 0xff && data[1] == 0xfe:
		start = 2
	case data[0] == 0xfe && data[1] == 0xff:
		order = binary.BigEndian
		start = 2
	case data[0] != 0 && data[1] == 0:
		// UTF-16LE without a byte-order mark.
	case data[0] == 0 && data[1] != 0:
		order = binary.BigEndian
	default:
		return data, nil
	}

	if (len(data)-start)%2 != 0 {
		return nil, fmt.Errorf("odd UTF-16 payload length")
	}

	units := make([]uint16, 0, (len(data)-start)/2)
	for index := start; index < len(data); index += 2 {
		units = append(units, order.Uint16(data[index:index+2]))
	}

	return []byte(string(utf16.Decode(units))), nil
}

func statusRank(status state.Status) int {
	switch status {
	case state.StatusMajor:
		return 3
	case state.StatusDegraded:
		return 2
	case state.StatusUnknown:
		return 1
	default:
		return 0
	}
}
