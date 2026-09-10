package main

import (
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

// TestHelperProcess is re-executed as a plugin backend subprocess by the
// proxy tests; it serves HTTP on the port core allocated for it.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("STAR_PANEL_TEST_HELPER") == "" {
		t.Skip("only runs as a supervised subprocess")
	}
	if os.Getenv("STAR_PANEL_PLUGIN") == "crash-demo" {
		os.Exit(1)
	}
	port := os.Getenv("STAR_PANEL_PORT")
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"plugin":%q,"path":%q}`, os.Getenv("STAR_PANEL_PLUGIN"), r.URL.Path)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such route"})
	})
	if err := http.ListenAndServe("127.0.0.1:"+port, mux); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func backendManifest(t *testing.T, pluginName string) map[string]string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find test binary: %v", err)
	}
	command := fmt.Sprintf(
		`{"name":%q,"version":"0.1.0","widgets":[{"id":"w","title":"W","module":"widget.js"}],"backend":{"command":[%q,"-test.run=TestHelperProcess","--"]}}`,
		pluginName, self,
	)
	return map[string]string{
		"manifest.json": command,
		"widget.js":     "export default () => {};",
	}
}

func newBackendTestServer(t *testing.T, filesByFolder map[string]map[string]string) (*httptest.Server, *plugins.Backends) {
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
	ts := httptest.NewServer(newServer(serverDeps{
		store:    store,
		registry: registry,
		backends: backends,
		web:      testDashboardFS(),
	}))
	t.Cleanup(ts.Close)
	return ts, backends
}

func getBody(t *testing.T, url string) (int, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer res.Body.Close()
	var buf strings.Builder
	raw := make([]byte, 1024)
	for {
		n, err := res.Body.Read(raw)
		buf.Write(raw[:n])
		if err != nil {
			break
		}
	}
	return res.StatusCode, buf.String()
}

func TestProxyPassthroughToSubprocess(t *testing.T) {
	t.Setenv("STAR_PANEL_TEST_HELPER", "1")
	ts, backends := newBackendTestServer(t, map[string]map[string]string{
		"backend-demo": backendManifest(t, "backend-demo"),
	})

	// Core startup: declared backends begin serving before any request.
	backends.Start()

	status, body := getBody(t, ts.URL+"/api/v1/plugins/backend-demo/proxy/ping")
	if status != http.StatusOK {
		t.Fatalf("proxy status = %d, body %s", status, body)
	}
	if !strings.Contains(body, `"plugin":"backend-demo"`) {
		t.Errorf("proxy body = %s", body)
	}
	if !strings.Contains(body, `"path":"/ping"`) {
		t.Errorf("proxy prefix not stripped, body = %s", body)
	}

	// Unknown backend routes pass the backend's own 404 through.
	status, _ = getBody(t, ts.URL+"/api/v1/plugins/backend-demo/proxy/nope")
	if status != http.StatusNotFound {
		t.Errorf("unknown backend route status = %d, want 404", status)
	}
}

func TestProxyCrashMapsToFriendlyError(t *testing.T) {
	t.Setenv("STAR_PANEL_TEST_HELPER", "1")
	ts, _ := newBackendTestServer(t, map[string]map[string]string{
		"crash-demo": backendManifest(t, "crash-demo"),
	})

	status, body := getBody(t, ts.URL+"/api/v1/plugins/crash-demo/proxy/ping")
	if status != http.StatusBadGateway {
		t.Fatalf("crashed backend proxy status = %d, body %s", status, body)
	}
	if !strings.Contains(body, "unavailable") {
		t.Errorf("expected widget-friendly error, got %s", body)
	}

	// A crashing plugin must not drop the dashboard.
	status, _ = getBody(t, ts.URL+"/api/v1/dashboard")
	if status != http.StatusOK {
		t.Errorf("dashboard during plugin crash status = %d, want 200", status)
	}
}

func TestFrontendOnlyPluginHasNoBackendAndRejectsProxy(t *testing.T) {
	ts, _ := newBackendTestServer(t, map[string]map[string]string{
		"hello-widget": {
			"manifest.json": `{"name":"hello-widget","version":"0.1.0","widgets":[{"id":"hello","title":"Hello","module":"widget.js"}]}`,
			"widget.js":     "export default () => {};",
		},
	})

	_, body := doJSON(t, ts, http.MethodGet, "/api/v1/plugins", nil)
	plugins, _ := body["plugins"].([]any)
	plugin, _ := plugins[0].(map[string]any)
	if plugin["backend"] != false {
		t.Errorf("frontend-only plugin listed backend = %v", plugin["backend"])
	}

	status, proxyBody := getBody(t, ts.URL+"/api/v1/plugins/hello-widget/proxy/anything")
	if status != http.StatusBadGateway {
		t.Fatalf("proxy on frontend-only plugin status = %d, body %s", status, proxyBody)
	}
	if !strings.Contains(proxyBody, "no backend") {
		t.Errorf("expected clear no-backend error, got %s", proxyBody)
	}
}

// A name no folder declares is the caller's mistake, not a backend that
// failed, and the two come back with different statuses.
func TestProxyOfUnknownPluginIsNotFound(t *testing.T) {
	ts, _ := newBackendTestServer(t, nil)

	status, body := getBody(t, ts.URL+"/api/v1/plugins/nope/proxy/ping")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, body %s, want 404", status, body)
	}
	if !strings.Contains(body, "plugin not found") {
		t.Errorf("body = %s, want a not-found error", body)
	}
}
