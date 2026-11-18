package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogInspectCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"page":{"name":"Example","url":"https://status.example.com"},
			"status":{"indicator":"none"},
			"components":[{"name":"API","status":"operational"}]
		}`)
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer

	root := newRootCommand(&rootOptions{httpClient: server.Client()})
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{
		"catalog", "inspect", server.URL + "/api/v2/summary.json",
		"--name", "Example", "--json",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var output struct {
		Result struct {
			Candidates []struct {
				Type string `json:"type"`
			} `json:"candidates"`
		} `json:"result"`
		Draft string `json:"draft_yaml"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	if len(output.Result.Candidates) == 0 || output.Result.Candidates[0].Type != "statuspage" {
		t.Errorf("candidates = %+v, want statuspage", output.Result.Candidates)
	}

	if !strings.Contains(output.Draft, "type: statuspage") || !strings.Contains(output.Draft, "name: Example") {
		t.Errorf("draft = %q, want statuspage Example", output.Draft)
	}
}

func TestCatalogInspectCommandHumanOutput(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"page":{"name":"Example","url":"https://status.example.com"},"status":{"indicator":"none"}}`)
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer

	root := newRootCommand(&rootOptions{httpClient: server.Client()})
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"catalog", "inspect", server.URL + "/api/v2/summary.json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(stdout.String(), "draft:") || !strings.Contains(stdout.String(), "statuspage") {
		t.Errorf("output = %q, want draft", stdout.String())
	}
}
