package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"star-panel/internal/dashboard"
	"star-panel/internal/plugins"
)

func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := dashboard.NewStore(dataDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	registry := plugins.NewRegistry(t.TempDir())
	backends := plugins.NewBackends(registry, nil)
	t.Cleanup(backends.Stop)
	return httptest.NewServer(newServer(serverDeps{
		store:    store,
		registry: registry,
		backends: backends,
		themes:   newThemeStore(t),
		web:      testDashboardFS(),
	})), dataDir
}

func doJSON(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	var decoded map[string]any
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return res, decoded
}

func TestDashboardGetReturnsDefaultShape(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/dashboard", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}

	if theme, _ := body["theme"].(string); theme != dashboard.DefaultTheme {
		t.Errorf("theme = %v, want %q", body["theme"], dashboard.DefaultTheme)
	}
	widgets, ok := body["widgets"].([]any)
	if !ok || len(widgets) == 0 {
		t.Fatalf("widgets missing or empty: %v", body["widgets"])
	}
	first, _ := widgets[0].(map[string]any)
	for _, key := range []string{"id", "plugin", "w", "h", "enabled", "pollSeconds"} {
		if _, present := first[key]; !present {
			t.Errorf("widget missing key %q", key)
		}
	}
	if _, present := first["size"]; present {
		t.Errorf("a Widget still advertises the retired size: %v", first)
	}
}

func TestDashboardSaveGetRoundTrip(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	saved := dashboard.Dashboard{
		Widgets: []dashboard.Widget{
			{ID: "clock", Plugin: "sysmon", Span: dashboard.Span{W: 12, H: 2}, Enabled: true, Config: json.RawMessage(`{"tz":"UTC"}`)},
			{ID: "notes", Plugin: "sysmon", Span: dashboard.Span{W: 4, H: 1}, Enabled: false},
		},
		Theme: "default",
	}
	res, _ := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", saved)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d, want 200", res.StatusCode)
	}

	// Save must default the unset poll interval.
	saved.Widgets[0].PollSeconds = dashboard.DefaultPollSeconds
	saved.Widgets[1].PollSeconds = dashboard.DefaultPollSeconds

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/dashboard", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", res.StatusCode)
	}
	raw, _ := json.Marshal(body)
	var got dashboard.Dashboard
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("redecode dashboard: %v", err)
	}
	if fmt.Sprint(got) != fmt.Sprint(saved) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, saved)
	}
	if len(got.Widgets) != 2 || got.Widgets[0].ID != "clock" {
		t.Errorf("widget order not preserved: %+v", got.Widgets)
	}
}

func TestDashboardSaveRejectsInvalidDocuments(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	cases := map[string]dashboard.Dashboard{
		"empty widget id": {
			Widgets: []dashboard.Widget{{ID: "", Plugin: "sysmon", Span: dashboard.Span{W: 4, H: 1}, Enabled: true}},
		},
		"duplicate widget ids": {
			Widgets: []dashboard.Widget{
				{ID: "a", Plugin: "sysmon", Span: dashboard.Span{W: 4, H: 1}, Enabled: true},
				{ID: "a", Plugin: "sysmon", Span: dashboard.Span{W: 4, H: 1}, Enabled: true},
			},
		},
		"wider than the grid": {
			Widgets: []dashboard.Widget{{ID: "a", Plugin: "sysmon", Span: dashboard.Span{W: 13, H: 1}, Enabled: true}},
		},
		"no rows": {
			Widgets: []dashboard.Widget{{ID: "a", Plugin: "sysmon", Span: dashboard.Span{W: 6, H: 0}, Enabled: true}},
		},
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			res, body := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", doc)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", res.StatusCode)
			}
			if _, ok := body["error"]; !ok {
				t.Errorf("expected error field in body, got %v", body)
			}
		})
	}
}

func TestDashboardSaveRejectsMalformedJSON(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/dashboard", bytes.NewBufferString("{not json"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

// The frontend keeps the PUT response in memory until the next poll, so the
// answer has to be the document as stored, defaults and all.
func TestDashboardSaveAnswersWithTheStoredDocument(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	sent := dashboard.Dashboard{
		Widgets: []dashboard.Widget{{ID: "solo", Plugin: "sysmon", Span: dashboard.Span{W: 4, H: 1}, Enabled: true}},
		Theme:   "default",
	}
	res, body := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", sent)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	raw, _ := json.Marshal(body)
	var got dashboard.Dashboard
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Widgets) != 1 {
		t.Fatalf("widgets = %+v, want one", got.Widgets)
	}
	if got.Widgets[0].PollSeconds != dashboard.DefaultPollSeconds {
		t.Errorf("pollSeconds = %d, want the default %d", got.Widgets[0].PollSeconds, dashboard.DefaultPollSeconds)
	}
}

// A document that cannot validate is the operator's problem to see: serving it
// would hand the frontend a Dashboard it cannot render.
func TestDashboardGetFailsLoudlyOnAnInvalidStoredDocument(t *testing.T) {
	ts, dataDir := newTestServer(t)
	defer ts.Close()

	broken := `{"widgets":[{"id":"a","plugin":"sysmon","size":"huge","enabled":true,"pollSeconds":10}],` +
		`"theme":"default"}`
	if err := os.WriteFile(filepath.Join(dataDir, "dashboard.json"), []byte(broken), 0o644); err != nil {
		t.Fatalf("write broken document: %v", err)
	}

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/dashboard", nil)
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	message, _ := body["error"].(string)
	if !strings.Contains(message, "w and h") {
		t.Errorf("error = %q, want the validation reason", message)
	}
}

// A tab opened before Spans PUTs the old shape. It is refused rather than
// guessed at, and the answer names the fields to send instead (ADR-0008).
func TestDashboardSaveRefusesTheRetiredSizeField(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	legacy := bytes.NewBufferString(`{"widgets":[{"id":"a","plugin":"sysmon","size":"medium",` +
		`"enabled":true,"pollSeconds":10}],"theme":"default"}`)
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/dashboard", legacy)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(body["error"], "w and h") {
		t.Errorf("error = %q, want the fields to send named", body["error"])
	}
}

func TestDashboardPersistsAcrossRestart(t *testing.T) {
	ts, dataDir := newTestServer(t)

	saved := dashboard.Dashboard{
		Widgets: []dashboard.Widget{{ID: "solo", Plugin: "sysmon", Span: dashboard.Span{W: 6, H: 1}, Enabled: false}},
		Theme:   "dracula",
	}
	res, _ := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", saved)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d, want 200", res.StatusCode)
	}
	ts.Close()

	restarted, err := dashboard.NewStore(dataDir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	got, err := restarted.Load()
	if err != nil {
		t.Fatalf("load after restart: %v", err)
	}
	saved.Widgets[0].PollSeconds = dashboard.DefaultPollSeconds
	if fmt.Sprint(got) != fmt.Sprint(saved) {
		t.Errorf("dashboard lost across restart:\n got %+v\nwant %+v", got, saved)
	}
}
