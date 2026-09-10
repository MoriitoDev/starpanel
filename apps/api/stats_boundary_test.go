package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func newBuiltinTestServer(t *testing.T, builtins map[string]http.Handler) (*httptest.Server, *PluginRegistry) {
	t.Helper()
	pluginsDir := t.TempDir()
	writeFiles(t, filepath.Join(pluginsDir, "system-stats"), map[string]string{
		"manifest.json": `{"name":"system-stats","version":"0.1.0","widgets":[{"id":"system-stats","title":"System Stats","module":"widget.js"}]}`,
		"widget.js":     "export default () => {};",
	})
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	registry := NewPluginRegistry(pluginsDir)
	backends := NewBackends(registry, builtins)
	t.Cleanup(backends.Stop)
	ts := httptest.NewServer(newMux(store, registry, backends))
	t.Cleanup(ts.Close)
	return ts, registry
}

func TestBuiltinPluginServesStatsThroughProxy(t *testing.T) {
	ts, _ := newBuiltinTestServer(t, map[string]http.Handler{
		"system-stats": newStatsHandler(stubStatsSource{}),
	})

	// Shape: cpu + memory + disk via the versioned plugin proxy.
	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/plugins/system-stats/proxy/stats", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("stats status = %d, body %v", res.StatusCode, body)
	}
	for _, key := range []string{"source", "cpuPercent", "memTotalBytes", "memUsedBytes", "diskTotalBytes", "diskUsedBytes"} {
		if _, ok := body[key]; !ok {
			t.Errorf("stats missing key %q: %v", key, body)
		}
	}
	if body["source"] != "stub" {
		t.Errorf("source = %v, want stub", body["source"])
	}
	cpu, ok := body["cpuPercent"].(float64)
	if !ok || cpu < 0 || cpu > 100 {
		t.Errorf("cpuPercent = %v, want a percentage", body["cpuPercent"])
	}
	memUsed, _ := body["memUsedBytes"].(float64)
	memTotal, _ := body["memTotalBytes"].(float64)
	if memUsed <= 0 || memUsed > memTotal {
		t.Errorf("memory usage %v not within total %v", memUsed, memTotal)
	}

	// Unknown built-in route stays a clean 404.
	res, _ = doJSON(t, ts, http.MethodGet, "/api/v1/plugins/system-stats/proxy/nope", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown builtin route status = %d, want 404", res.StatusCode)
	}
}

func TestBuiltinPluginAppearsInListingWithoutBackend(t *testing.T) {
	ts, _ := newBuiltinTestServer(t, map[string]http.Handler{
		"system-stats": newStatsHandler(stubStatsSource{}),
	})

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/plugins", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("listing status = %d", res.StatusCode)
	}
	plugins, _ := body["plugins"].([]any)
	found := false
	for _, p := range plugins {
		plugin, _ := p.(map[string]any)
		if plugin["name"] == "system-stats" {
			found = true
			if plugin["backend"] != false {
				t.Errorf("builtin listed backend = %v, want false", plugin["backend"])
			}
		}
	}
	if !found {
		t.Errorf("system-stats not in listing: %v", body)
	}
}

func TestStatsHandlerReportsSourceErrors(t *testing.T) {
	failing := &linuxStatsSource{
		read:   func(path string) ([]byte, error) { return nil, errInjected },
		statfs: func(path string) (*statfsInfo, error) { return nil, errInjected },
	}
	ts, _ := newBuiltinTestServer(t, map[string]http.Handler{
		"system-stats": newStatsHandler(failing),
	})

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/plugins/system-stats/proxy/stats", nil)
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("failing source status = %d, want 500", res.StatusCode)
	}
	if msg, _ := body["error"].(string); msg == "" {
		t.Errorf("expected error message, got %v", body)
	}
}

var errInjected = &injectedError{}

type injectedError struct{}

func (*injectedError) Error() string { return "injected failure" }
