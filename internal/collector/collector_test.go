package collector_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/collector"
	"github.com/fabianoflorentino/seraph/internal/state"
)

var fixedNow = time.Date(2026, time.November, 18, 12, 0, 0, 0, time.UTC)

func options(extra ...collector.Option) []collector.Option {
	opts := []collector.Option{
		collector.WithRetries(0),
		collector.WithBackoff(func(int) time.Duration { return 0 }),
		collector.WithClock(func() time.Time { return fixedNow }),
	}

	return append(opts, extra...)
}

func service(slug, url, component string) catalog.Service {
	return catalog.Service{
		Slug:        slug,
		Name:        "GitHub",
		Category:    catalog.CategoryCloud,
		OfficialURL: "https://github.com",
		StatusURL:   "https://www.githubstatus.com",
		Regions:     map[string]catalog.Level{"global": catalog.LevelPrimary},
		Source: catalog.Source{
			Type:      catalog.SourceStatuspage,
			URL:       url,
			Component: component,
		},
	}
}

func statusServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	return server
}

func collectOne(t *testing.T, entry catalog.Service, extra ...collector.Option) state.Service {
	t.Helper()

	c := collector.New(nil, options(extra...)...)

	results := c.Collect(context.Background(), []catalog.Service{entry})
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	return results[0]
}

func TestCollectorMapsStatuspageIndicators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		indicator string
		want      state.Status
		wantError bool
	}{
		{name: "operational", indicator: "none", want: state.StatusOperational},
		{name: "minor", indicator: "minor", want: state.StatusDegraded},
		{name: "major", indicator: "major", want: state.StatusMajor},
		{name: "critical", indicator: "critical", want: state.StatusMajor},
		{name: "maintenance", indicator: "maintenance", want: state.StatusDegraded},
		{name: "unknown", indicator: "weird", want: state.StatusUnknown, wantError: true},
		{name: "empty", indicator: "", want: state.StatusUnknown, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := statusServer(t, fmt.Sprintf(`{"status":{"indicator":%q}}`, test.indicator))
			result := collectOne(t, service("github", server.URL, ""))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if result.StatusURL != "https://www.githubstatus.com" {
				t.Errorf("StatusURL = %q, want status page", result.StatusURL)
			}

			if test.wantError {
				if result.Error == "" {
					t.Error("Error is empty, want a collection error")
				}

				if len(result.Points) != 0 {
					t.Errorf("Points = %d, want 0 on failure", len(result.Points))
				}

				return
			}

			if result.Error != "" {
				t.Errorf("Error = %q, want empty", result.Error)
			}

			if result.ObservedAt == nil || !result.ObservedAt.Equal(fixedNow) {
				t.Errorf("ObservedAt = %v, want %v", result.ObservedAt, fixedNow)
			}

			if len(result.Points) != 1 || result.Points[0].Status != test.want {
				t.Errorf("Points = %+v, want one point with %q", result.Points, test.want)
			}
		})
	}
}

func TestCollectorUsesComponentStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component string
		health    string
		want      state.Status
		wantError bool
	}{
		{name: "operational", component: "API", health: "operational", want: state.StatusOperational},
		{name: "degraded performance", component: "API", health: "degraded_performance", want: state.StatusDegraded},
		{name: "partial outage", component: "API", health: "partial_outage", want: state.StatusDegraded},
		{name: "maintenance", component: "API", health: "under_maintenance", want: state.StatusDegraded},
		{name: "major outage", component: "API", health: "major_outage", want: state.StatusMajor},
		{name: "unknown status", component: "API", health: "mystery", want: state.StatusUnknown, wantError: true},
		{name: "empty status", component: "API", health: "", want: state.StatusUnknown, wantError: true},
		{name: "missing component", component: "Web", want: state.StatusUnknown, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(
				`{"status":{"indicator":"major"},"components":[{"name":"API","status":%q}]}`,
				test.health,
			)
			server := statusServer(t, body)
			result := collectOne(t, service("github", server.URL, test.component))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if (result.Error != "") != test.wantError {
				t.Errorf("Error = %q, want error %t", result.Error, test.wantError)
			}
		})
	}
}

func TestCollectorRetriesServerErrors(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		_, _ = io.WriteString(w, `{"status":{"indicator":"none"}}`)
	}))
	t.Cleanup(server.Close)

	result := collectOne(t, service("github", server.URL, ""), collector.WithRetries(2))

	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}

	if result.Status != state.StatusOperational {
		t.Errorf("Status = %q, want operational", result.Status)
	}
}

func TestCollectorStopsAfterRetryBudget(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	result := collectOne(t, service("github", server.URL, ""), collector.WithRetries(2))

	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}

	if result.Status != state.StatusUnknown {
		t.Errorf("Status = %q, want unknown", result.Status)
	}

	if !strings.Contains(result.Error, "502") {
		t.Errorf("Error = %q, want HTTP 502", result.Error)
	}
}

func TestCollectorDoesNotRetryClientErrors(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	result := collectOne(t, service("github", server.URL, ""), collector.WithRetries(2))

	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}

	if result.Status != state.StatusUnknown {
		t.Errorf("Status = %q, want unknown", result.Status)
	}

	if !strings.Contains(result.Error, "404") {
		t.Errorf("Error = %q, want HTTP 404", result.Error)
	}
}

func TestCollectorHandlesInvalidPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "invalid json", body: "not json", wantErr: "decode status page"},
		{name: "too large", body: strings.Repeat(" ", (2<<20)+1), wantErr: "too large"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := statusServer(t, test.body)
			result := collectOne(t, service("github", server.URL, ""))

			if result.Status != state.StatusUnknown {
				t.Errorf("Status = %q, want unknown", result.Status)
			}

			if !strings.Contains(result.Error, test.wantErr) {
				t.Errorf("Error = %q, want %q", result.Error, test.wantErr)
			}
		})
	}
}

func TestCollectorHonorsTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(w, `{"status":{"indicator":"none"}}`)
	}))
	t.Cleanup(server.Close)

	result := collectOne(t, service("github", server.URL, ""), collector.WithTimeout(10*time.Millisecond))

	if result.Status != state.StatusUnknown {
		t.Errorf("Status = %q, want unknown", result.Status)
	}

	if result.Error == "" {
		t.Error("Error is empty, want a timeout error")
	}
}

func TestCollectorRejectsUnsupportedSource(t *testing.T) {
	t.Parallel()

	entry := service("github", "https://example.com/status", "")
	entry.Source.Type = catalog.SourceProbe

	result := collectOne(t, entry)

	if result.Status != state.StatusUnknown {
		t.Errorf("Status = %q, want unknown", result.Status)
	}

	if !strings.Contains(result.Error, "not implemented") {
		t.Errorf("Error = %q, want unimplemented source message", result.Error)
	}
}

func TestCollectorStopsWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := collector.New(nil, options()...)
	results := c.Collect(ctx, []catalog.Service{service("github", "https://example.com/status", "")})

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Status != state.StatusUnknown {
		t.Errorf("Status = %q, want unknown", results[0].Status)
	}

	if results[0].Error == "" {
		t.Error("Error is empty, want a cancellation error")
	}
}

func TestCollectorPreservesOrderAndIsolatesFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			_, _ = io.WriteString(w, `{"status":{"indicator":"none"}}`)

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	services := []catalog.Service{
		service("alpha", server.URL+"/ok", ""),
		service("beta", server.URL+"/fail", ""),
		service("gamma", server.URL+"/ok", ""),
	}

	c := collector.New(nil, options(collector.WithConcurrency(2))...)
	results := c.Collect(context.Background(), services)

	if len(results) != len(services) {
		t.Fatalf("results = %d, want %d", len(results), len(services))
	}

	for index, slug := range []string{"alpha", "beta", "gamma"} {
		if results[index].Slug != slug {
			t.Errorf("results[%d].Slug = %q, want %q", index, results[index].Slug, slug)
		}
	}

	if results[0].Status != state.StatusOperational || results[2].Status != state.StatusOperational {
		t.Errorf("healthy statuses = %q, %q, want operational", results[0].Status, results[2].Status)
	}

	if results[1].Status != state.StatusUnknown || results[1].Error == "" {
		t.Errorf("failed result = %+v, want unknown with error", results[1])
	}

	if results[1].Name != "GitHub" ||
		results[1].OfficialURL != "https://github.com" ||
		results[1].StatusURL != "https://www.githubstatus.com" {
		t.Errorf("failure metadata = %+v, want catalog metadata preserved", results[1])
	}
}
