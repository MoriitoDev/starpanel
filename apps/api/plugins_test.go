package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"star-panel/internal/dashboard"
	"star-panel/internal/plugins"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func newTestServerWithPlugins(t *testing.T, filesByFolder map[string]map[string]string) *httptest.Server {
	t.Helper()
	pluginsDir := t.TempDir()
	for folder, files := range filesByFolder {
		writeFiles(t, filepath.Join(pluginsDir, folder), files)
	}
	store, err := dashboard.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	registry := plugins.NewRegistry(pluginsDir)
	backends := plugins.NewBackends(registry, nil)
	t.Cleanup(backends.Stop)
	return httptest.NewServer(newServer(serverDeps{
		store:    store,
		registry: registry,
		backends: backends,
		themes:   newThemeStore(t),
		web:      testDashboardFS(),
	}))
}

func TestPluginListingShowsValidAndReportsInvalid(t *testing.T) {
	ts := newTestServerWithPlugins(t, map[string]map[string]string{
		"hello-widget": {
			"manifest.json": `{"name":"hello-widget","version":"0.1.0","widgets":[{"id":"hello","title":"Hello","module":"widget.js"}]}`,
			"widget.js":     "export default () => {};",
		},
		"broken": {"manifest.json": "{not json"},
	})
	defer ts.Close()

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/plugins", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	plugins, _ := body["plugins"].([]any)
	if len(plugins) != 1 {
		t.Fatalf("plugins = %v, want exactly one valid plugin", plugins)
	}
	plugin, _ := plugins[0].(map[string]any)
	if plugin["name"] != "hello-widget" {
		t.Errorf("plugin name = %v", plugin["name"])
	}
	if plugin["backend"] != false {
		t.Errorf("frontend-only plugin listed backend = %v", plugin["backend"])
	}
	widgets, _ := plugin["widgets"].([]any)
	first, _ := widgets[0].(map[string]any)
	module, _ := first["module"].(string)
	if !strings.HasPrefix(module, "/api/v1/plugins/hello-widget/modules/") {
		t.Errorf("module URL = %q", module)
	}
	errors, _ := body["errors"].([]any)
	if len(errors) != 1 {
		t.Fatalf("errors = %v, want exactly one rejected folder", errors)
	}
	errEntry, _ := errors[0].(map[string]any)
	if errEntry["folder"] != "broken" {
		t.Errorf("error folder = %v", errEntry["folder"])
	}
	if msg, _ := errEntry["error"].(string); msg == "" {
		t.Errorf("expected a clear error message, got %v", errEntry)
	}
}

func TestPluginListingSurvivesEmptyDir(t *testing.T) {
	ts := newTestServerWithPlugins(t, nil)
	defer ts.Close()

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/plugins", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if plugins, _ := body["plugins"].([]any); len(plugins) != 0 {
		t.Errorf("plugins = %v, want empty", plugins)
	}
}

func TestPluginModuleServing(t *testing.T) {
	ts := newTestServerWithPlugins(t, map[string]map[string]string{
		"hello-widget": {
			"manifest.json": `{"name":"hello-widget","version":"0.1.0","widgets":[{"id":"hello","title":"Hello","module":"widget.js"}]}`,
			"widget.js":     "export default function render(el, ctx) { el.hello = true; }",
		},
	})
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/plugins/hello-widget/modules/widget.js", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("get module: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("content type = %q, want text/javascript", ct)
	}
	buf := make([]byte, 128)
	n, _ := res.Body.Read(buf)
	if !strings.Contains(string(buf[:n]), "export default") {
		t.Errorf("module body = %q", string(buf[:n]))
	}

	// Unknown plugin and unknown module map to 404 JSON, not a crash.
	for _, path := range []string{
		"/api/v1/plugins/nope/modules/widget.js",
		"/api/v1/plugins/hello-widget/modules/missing.js",
	} {
		res, err := ts.Client().Get(ts.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, res.StatusCode)
		}
	}
}
