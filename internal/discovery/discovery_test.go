package discovery_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fabianoflorentino/seraph/internal/discovery"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/summary.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"page": {"name": "Example", "url": "https://status.example.com", "updated_at": "2026-11-18T17:00:00Z"},
			"status": {"indicator": "none", "description": "All Systems Operational"},
			"components": [{"name": "API", "status": "operational"}]
		}`))
	})
	mux.HandleFunc("/1.0/status/abc123", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"status_overall":{"status":"Operational","status_code":100}}}`))
	})
	mux.HandleFunc("/incidents.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"begin":"2026-11-18T10:00:00Z","external_desc":"test"}]`))
	})
	mux.HandleFunc("/history.rss", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel></channel></rss>`))
	})
	mux.HandleFunc("/html", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>Powered by Statuspage</body></html>`))
	})
	mux.HandleFunc("/statusio", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/pages/5b36dc6502d06804c08349f7">Status</a></body></html>`))
	})
	mux.HandleFunc("/1.0/status/5b36dc6502d06804c08349f7", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"status_overall":{"status":"Operational","status_code":100}}}`))
	})

	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	return server
}

func TestInspectDetectsSources(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	inspector := discovery.New(server.Client())

	tests := []struct {
		name       string
		path       string
		wantType   string
		wantFormat string
	}{
		{name: "statuspage", path: "/api/v2/summary.json", wantType: "statuspage"},
		{name: "statusio", path: "/1.0/status/abc123", wantType: "statusio"},
		{name: "gcp", path: "/incidents.json", wantType: "json", wantFormat: "gcp"},
		{name: "rss", path: "/history.rss", wantType: "rss"},
		{name: "html", path: "/html", wantType: "html"},
		{name: "statusio html", path: "/statusio", wantType: "statusio"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := inspector.Inspect(context.Background(), server.URL+test.path)
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}

			if len(result.Candidates) == 0 {
				t.Fatal("no candidates")
			}

			found := false
			for _, candidate := range result.Candidates {
				if candidate.Type == test.wantType && (test.wantFormat == "" || candidate.Format == test.wantFormat) {
					found = true

					if candidate.LatencyMS < 0 {
						t.Errorf("LatencyMS = %d, want non-negative", candidate.LatencyMS)
					}
				}
			}

			if !found {
				t.Errorf("candidates = %+v, want %s/%s", result.Candidates, test.wantType, test.wantFormat)
			}
		})
	}
}

func TestInspectReturnsErrNoSources(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	inspector := discovery.New(server.Client())

	_, err := inspector.Inspect(context.Background(), server.URL+"/missing")
	if !errors.Is(err, discovery.ErrNoSources) {
		t.Fatalf("error = %v, want ErrNoSources", err)
	}
}
