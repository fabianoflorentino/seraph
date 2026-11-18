package collector_test

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/state"
)

func typedService(url string, sourceType catalog.SourceType, component, format string) catalog.Service {
	entry := service("service", url, component)
	entry.Source.Type = sourceType
	entry.Source.Format = format

	return entry
}

func jsonServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	return server
}

func TestCollectorMapsStatusIO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component string
		body      string
		want      state.Status
		wantError bool
	}{
		{
			name: "overall operational",
			body: `{"result":{"status_overall":{"status":"Operational","status_code":100}}}`,
			want: state.StatusOperational,
		},
		{
			name: "overall maintenance",
			body: `{"result":{"status_overall":{"status":"Planned Maintenance","status_code":200}}}`,
			want: state.StatusDegraded,
		},
		{
			name: "overall major",
			body: `{"result":{"status_overall":{"status":"Service Disruption","status_code":500}}}`,
			want: state.StatusMajor,
		},
		{
			name:      "component operational",
			component: "API",
			body:      `{"result":{"status_overall":{"status":"Operational","status_code":100},"status":[{"name":"API","status":"Operational","status_code":100}]}}`,
			want:      state.StatusOperational,
		},
		{
			name:      "component degraded",
			component: "API",
			body:      `{"result":{"status_overall":{"status":"Operational","status_code":100},"status":[{"name":"API","status":"Degraded Performance","status_code":300}]}}`,
			want:      state.StatusDegraded,
		},
		{
			name:      "missing component",
			component: "Web",
			body:      `{"result":{"status_overall":{"status":"Operational","status_code":100}}}`,
			want:      state.StatusUnknown,
			wantError: true,
		},
		{
			name:      "unknown component code",
			component: "API",
			body:      `{"result":{"status":[{"name":"API","status":"Mystery","status_code":999}]}}`,
			want:      state.StatusUnknown,
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := jsonServer(t, test.body)
			result := collectOne(t, typedService(server.URL, catalog.SourceStatusIO, test.component, ""))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if (result.Error != "") != test.wantError {
				t.Errorf("Error = %q, want error %t", result.Error, test.wantError)
			}
		})
	}
}

func TestCollectorStatusIOStatusLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label string
		want  state.Status
	}{
		{label: "Operational", want: state.StatusOperational},
		{label: "Planned Maintenance", want: state.StatusDegraded},
		{label: "Under Maintenance", want: state.StatusDegraded},
		{label: "Partial Service Disruption", want: state.StatusDegraded},
		{label: "Security Event", want: state.StatusDegraded},
		{label: "Service Disruption", want: state.StatusMajor},
		{label: "Major Outage", want: state.StatusMajor},
	}

	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(`{"result":{"status_overall":{"status":%q}}}`, test.label)
			server := jsonServer(t, body)
			result := collectOne(t, typedService(server.URL, catalog.SourceStatusIO, "", ""))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}
		})
	}
}

func TestCollectorStatusIORejectsUnknownPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "invalid json", status: http.StatusOK, body: "not json", wantErr: "decode status.io"},
		{name: "http error", status: http.StatusBadGateway, body: "", wantErr: "502"},
		{name: "empty status", status: http.StatusOK, body: `{"result":{"status_overall":{}}}`, wantErr: "empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			t.Cleanup(server.Close)

			result := collectOne(t, typedService(server.URL, catalog.SourceStatusIO, "", ""))

			if result.Status != state.StatusUnknown {
				t.Errorf("Status = %q, want unknown", result.Status)
			}

			if !strings.Contains(result.Error, test.wantErr) {
				t.Errorf("Error = %q, want %q", result.Error, test.wantErr)
			}
		})
	}
}

func TestCollectorMapsSlackStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      state.Status
		wantError bool
	}{
		{name: "operational", body: `{"status":"ok","active_incidents":[]}`, want: state.StatusOperational},
		{name: "active incident", body: `{"status":"ok","active_incidents":[{"id":"1"}]}`, want: state.StatusDegraded},
		{name: "notice", body: `{"status":"notice"}`, want: state.StatusDegraded},
		{name: "critical", body: `{"status":"critical"}`, want: state.StatusMajor},
		{name: "unknown status", body: `{"status":"mystery"}`, want: state.StatusUnknown, wantError: true},
		{name: "empty status", body: `{}`, want: state.StatusUnknown, wantError: true},
		{name: "invalid json", body: `not json`, want: state.StatusUnknown, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := jsonServer(t, test.body)
			result := collectOne(t, typedService(server.URL, catalog.SourceJSON, "", catalog.JSONFormatSlack))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if (result.Error != "") != test.wantError {
				t.Errorf("Error = %q, want error %t", result.Error, test.wantError)
			}
		})
	}
}

func TestCollectorMapsStripeStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      state.Status
		wantError bool
	}{
		{name: "up", body: `{"largestatus":"up","statuses":{"api":"up"}}`, want: state.StatusOperational},
		{name: "degraded", body: `{"largestatus":"degraded","statuses":{"api":"degraded"}}`, want: state.StatusDegraded},
		{name: "down", body: `{"largestatus":"down","statuses":{"api":"down"}}`, want: state.StatusMajor},
		{name: "worst component", body: `{"statuses":{"api":"up","checkout":"down"}}`, want: state.StatusMajor},
		{name: "unknown status", body: `{"largestatus":"mystery"}`, want: state.StatusUnknown, wantError: true},
		{name: "empty", body: `{}`, want: state.StatusUnknown, wantError: true},
		{name: "invalid json", body: `not json`, want: state.StatusUnknown, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := jsonServer(t, test.body)
			result := collectOne(t, typedService(server.URL, catalog.SourceJSON, "", catalog.JSONFormatStripe))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if (result.Error != "") != test.wantError {
				t.Errorf("Error = %q, want error %t", result.Error, test.wantError)
			}
		})
	}
}

func TestCollectorRejectsUnknownJSONFormat(t *testing.T) {
	t.Parallel()

	server := jsonServer(t, `{"status":"ok"}`)
	result := collectOne(t, typedService(server.URL, catalog.SourceJSON, "", "mystery"))

	if result.Status != state.StatusUnknown {
		t.Errorf("Status = %q, want unknown", result.Status)
	}

	if !strings.Contains(result.Error, "unsupported json format") {
		t.Errorf("Error = %q, want unsupported format", result.Error)
	}
}

func rawServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	return server
}

func utf16LE(value string) []byte {
	units := utf16.Encode([]rune(value))
	body := make([]byte, 2, 2+len(units)*2)
	body[0] = 0xff
	body[1] = 0xfe

	for _, unit := range units {
		body = binary.LittleEndian.AppendUint16(body, unit)
	}

	return body
}

func TestCollectorMapsAWSStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      state.Status
		wantError bool
	}{
		{name: "empty", body: `[]`, want: state.StatusOperational},
		{name: "resolved", body: `[{"status":"3"}]`, want: state.StatusOperational},
		{name: "investigating", body: `[{"status":"1"}]`, want: state.StatusDegraded},
		{name: "in progress", body: `[{"status":"2"}]`, want: state.StatusDegraded},
		{name: "unknown status", body: `[{"status":"9"}]`, want: state.StatusUnknown, wantError: true},
		{name: "invalid json", body: `not json`, want: state.StatusUnknown, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := rawServer(t, utf16LE(test.body))
			result := collectOne(t, typedService(server.URL, catalog.SourceJSON, "", catalog.JSONFormatAWS))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if (result.Error != "") != test.wantError {
				t.Errorf("Error = %q, want error %t", result.Error, test.wantError)
			}
		})
	}
}

func TestCollectorMapsGCPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      state.Status
		wantError bool
	}{
		{name: "empty", body: `[]`, want: state.StatusOperational},
		{name: "active", body: `[{"begin":"2026-11-18T10:00:00Z","end":""}]`, want: state.StatusDegraded},
		{name: "future end", body: `[{"begin":"2026-11-18T10:00:00Z","end":"2026-11-19T10:00:00Z"}]`, want: state.StatusDegraded},
		{name: "resolved", body: `[{"begin":"2026-11-17T10:00:00Z","end":"2026-11-17T12:00:00Z"}]`, want: state.StatusOperational},
		{name: "invalid end", body: `[{"end":"not-a-time"}]`, want: state.StatusUnknown, wantError: true},
		{name: "invalid json", body: `not json`, want: state.StatusUnknown, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := jsonServer(t, test.body)
			result := collectOne(t, typedService(server.URL, catalog.SourceJSON, "", catalog.JSONFormatGCP))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if (result.Error != "") != test.wantError {
				t.Errorf("Error = %q, want error %t", result.Error, test.wantError)
			}
		})
	}
}

func TestCollectorMapsRSS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      state.Status
		wantError bool
	}{
		{name: "empty", body: `<rss><channel><title>Status</title></channel></rss>`, want: state.StatusOperational},
		{name: "incident", body: `<rss><channel><item><title>Incident</title></item></channel></rss>`, want: state.StatusDegraded},
		{name: "invalid xml", body: `not xml`, want: state.StatusUnknown, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := jsonServer(t, test.body)
			result := collectOne(t, typedService(server.URL, catalog.SourceRSS, "", ""))

			if result.Status != test.want {
				t.Errorf("Status = %q, want %q", result.Status, test.want)
			}

			if (result.Error != "") != test.wantError {
				t.Errorf("Error = %q, want error %t", result.Error, test.wantError)
			}
		})
	}
}
