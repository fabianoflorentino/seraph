// Package discovery finds machine-readable status endpoints for a service.
package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var statusIOPagePattern = regexp.MustCompile(`/pages/([0-9a-f]{24})`)

const (
	maxBodyBytes     = 2 << 20
	defaultTimeout   = 10 * time.Second
	defaultUserAgent = "seraph-status-discovery"
)

// Candidate is one detected status endpoint.
type Candidate struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	Format     string `json:"format,omitempty"`
	StatusURL  string `json:"status_url,omitempty"`
	Name       string `json:"name,omitempty"`
	Components int    `json:"components,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	LatencyMS  int64  `json:"latency_ms"`
}

// Result is the set of candidates found for a target.
type Result struct {
	Target     string      `json:"target"`
	Candidates []Candidate `json:"candidates"`
}

// ErrNoSources means no machine-readable status endpoint was found.
var ErrNoSources = errors.New("no status sources found")

// Inspector probes a target for known status endpoint shapes.
type Inspector struct {
	client    *http.Client
	timeout   time.Duration
	userAgent string
}

// Option customizes an Inspector.
type Option func(*Inspector)

// WithTimeout sets the per-request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(i *Inspector) {
		if timeout > 0 {
			i.timeout = timeout
		}
	}
}

// WithUserAgent overrides the HTTP user agent.
func WithUserAgent(userAgent string) Option {
	return func(i *Inspector) {
		if userAgent != "" {
			i.userAgent = userAgent
		}
	}
}

// New creates an Inspector. A nil client uses http.DefaultClient.
func New(client *http.Client, opts ...Option) *Inspector {
	if client == nil {
		client = http.DefaultClient
	}

	i := &Inspector{
		client:    client,
		timeout:   defaultTimeout,
		userAgent: defaultUserAgent,
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

// Inspect probes candidate URLs for target and returns recognized sources.
func (i *Inspector) Inspect(ctx context.Context, target string) (Result, error) {
	parsed, err := normalizeTarget(target)
	if err != nil {
		return Result{Target: target}, err
	}

	result := Result{Target: parsed.Host}
	seen := make(map[string]struct{})

	for _, candidateURL := range candidateURLs(parsed) {
		candidate, found := i.check(ctx, candidateURL)
		if !found {
			continue
		}

		key := candidate.Type + "|" + candidate.StatusURL
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		result.Candidates = append(result.Candidates, candidate)
	}

	if len(result.Candidates) == 0 {
		return result, ErrNoSources
	}

	return result, nil
}

func normalizeTarget(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("target is empty")
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse target: %w", err)
	}

	if parsed.Host == "" {
		return nil, errors.New("target has no host")
	}

	parsed.Scheme = "https"
	parsed.Fragment = ""

	return parsed, nil
}

func candidateURLs(target *url.URL) []string {
	host := target.Hostname()
	statusHost := host
	if !strings.HasPrefix(host, "status.") {
		statusHost = "status." + host
	}

	var urls []string
	add := func(raw string) {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" {
			return
		}

		parsed.Scheme = "https"
		parsed.Fragment = ""
		urls = appendUnique(urls, parsed.String())
	}

	if target.Path != "" && target.Path != "/" {
		add(target.String())
	}

	add(fmt.Sprintf("https://%s/api/v2/summary.json", statusHost))
	add(fmt.Sprintf("https://%s/api/v2/summary.json", host))
	add(fmt.Sprintf("https://%s/api/v1/summary", statusHost))
	add(fmt.Sprintf("https://%s/api/v2/status.json", statusHost))
	add(fmt.Sprintf("https://%s/history.rss", statusHost))
	add(fmt.Sprintf("https://%s/", statusHost))
	add(fmt.Sprintf("https://%s/status", host))
	add(fmt.Sprintf("https://%s/status.rss", host))

	return urls
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}

func (i *Inspector) check(ctx context.Context, candidateURL string) (Candidate, bool) {
	requestCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, candidateURL, nil)
	if err != nil {
		return Candidate{}, false
	}

	request.Header.Set("Accept", "application/json, application/xml, text/xml, text/html")
	request.Header.Set("User-Agent", i.userAgent)

	start := time.Now()

	response, err := i.client.Do(request)
	if err != nil {
		return Candidate{}, false
	}

	defer func() { _ = response.Body.Close() }()

	latency := time.Since(start)

	if response.StatusCode != http.StatusOK {
		return Candidate{}, false
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes {
		return Candidate{}, false
	}

	candidate, ok := detect(candidateURL, body)
	if !ok {
		return Candidate{}, false
	}

	candidate.LatencyMS = latency.Milliseconds()

	return candidate, true
}

func detect(sourceURL string, body []byte) (Candidate, bool) {
	origin := sourceOrigin(sourceURL)
	lower := strings.ToLower(string(body))

	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		if candidate, ok := detectJSON(sourceURL, origin, payload); ok {
			return candidate, true
		}
	}

	if strings.Contains(lower, "<rss") || strings.Contains(lower, "<feed") {
		return Candidate{
			URL:       sourceURL,
			Type:      "rss",
			StatusURL: origin,
			Name:      origin,
		}, true
	}

	if strings.Contains(lower, "status.io") || statusIOPagePattern.Match(body) {
		if match := statusIOPagePattern.FindSubmatch(body); len(match) == 2 {
			return Candidate{
				URL:       origin + "/1.0/status/" + string(match[1]),
				Type:      "statusio",
				StatusURL: origin,
			}, true
		}
	}

	if strings.Contains(lower, "statuspage") || strings.Contains(lower, "incident.io") {
		return detectHTML(origin)
	}

	return Candidate{}, false
}

func detectJSON(sourceURL, origin string, payload any) (Candidate, bool) {
	switch value := payload.(type) {
	case map[string]any:
		if page, ok := value["page"].(map[string]any); ok {
			if status, ok := value["status"].(map[string]any); ok && stringValue(status["indicator"]) != "" {
				components, _ := value["components"].([]any)
				statusURL := stringValue(page["url"])
				if statusURL == "" {
					statusURL = origin
				}

				return Candidate{
					URL:        sourceURL,
					Type:       "statuspage",
					StatusURL:  statusURL,
					Name:       stringValue(page["name"]),
					Components: len(components),
					UpdatedAt:  stringValue(page["updated_at"]),
				}, true
			}
		}

		if result, ok := value["result"].(map[string]any); ok {
			if overall, ok := result["status_overall"].(map[string]any); ok {
				if stringValue(overall["status"]) != "" || numberValue(overall["status_code"]) != 0 {
					return Candidate{
						URL:       sourceURL,
						Type:      "statusio",
						StatusURL: origin,
						Name:      origin,
					}, true
				}
			}
		}

		if _, ok := value["active_incidents"].([]any); ok && stringValue(value["status"]) != "" {
			return Candidate{
				URL:       sourceURL,
				Type:      "json",
				Format:    "slack",
				StatusURL: origin,
				Name:      origin,
			}, true
		}

		if stringValue(value["largestatus"]) != "" {
			if _, ok := value["statuses"].(map[string]any); ok {
				return Candidate{
					URL:       sourceURL,
					Type:      "json",
					Format:    "stripe",
					StatusURL: origin,
					Name:      origin,
				}, true
			}
		}
	case []any:
		if len(value) > 0 {
			if first, ok := value[0].(map[string]any); ok {
				if stringValue(first["begin"]) != "" && (stringValue(first["external_desc"]) != "" || first["updates"] != nil) {
					return Candidate{
						URL:       sourceURL,
						Type:      "json",
						Format:    "gcp",
						StatusURL: "https://status.cloud.google.com",
						Name:      "Google Cloud",
					}, true
				}
			}
		}
	}

	return Candidate{}, false
}

func detectHTML(origin string) (Candidate, bool) {
	return Candidate{
		URL:       origin,
		Type:      "html",
		StatusURL: origin,
		Name:      origin,
	}, true
}

func sourceOrigin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}

	return "https://" + parsed.Host
}

func stringValue(value any) string {
	text, _ := value.(string)

	return text
}

func numberValue(value any) int {
	number, _ := value.(float64)

	return int(number)
}
