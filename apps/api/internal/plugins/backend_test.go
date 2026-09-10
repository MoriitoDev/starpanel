package plugins

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelperProcess is re-executed as a supervised backend by the tests here:
// the same shape any Plugin backend has, without needing another toolchain.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("STAR_PANEL_TEST_HELPER") == "" {
		t.Skip("only runs as a supervised subprocess")
	}
	if os.Getenv("STAR_PANEL_PLUGIN") == "crash-demo" {
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"plugin":%q,"path":%q}`, os.Getenv("STAR_PANEL_PLUGIN"), r.URL.Path)
	})
	if err := http.ListenAndServe("127.0.0.1:"+os.Getenv("STAR_PANEL_PORT"), mux); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func writeBackendPlugin(t *testing.T, dir, name string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find test binary: %v", err)
	}
	writePluginFolder(t, dir, name, map[string]string{
		"manifest.json": fmt.Sprintf(
			`{"name":%q,"version":"0.1.0","widgets":[{"id":"w","title":"W","module":"widget.js"}],"backend":{"command":[%q,"-test.run=TestHelperProcess","--"]}}`,
			name, self,
		),
		"widget.js": "export default () => {};",
	})
}

func getFromHandler(t *testing.T, handler http.Handler, path string) (int, string) {
	t.Helper()
	ts := httptest.NewServer(handler)
	defer ts.Close()
	res, err := ts.Client().Get(ts.URL + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res.StatusCode, string(body)
}

func TestHandlerReachesASupervisedSubprocess(t *testing.T) {
	t.Setenv("STAR_PANEL_TEST_HELPER", "1")
	dir := t.TempDir()
	writeBackendPlugin(t, dir, "backend-demo")

	backends := NewBackends(NewRegistry(dir), nil)
	t.Cleanup(backends.Stop)

	handler, err := backends.Handler("backend-demo")
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	status, body := getFromHandler(t, handler, "/ping")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body %s", status, body)
	}
	if !strings.Contains(body, `"plugin":"backend-demo"`) {
		t.Errorf("body = %s, want the subprocess answer", body)
	}
}

// Start is what core calls at boot so a declared backend is already serving.
func TestStartAndHandlerShareTheSameSubprocess(t *testing.T) {
	t.Setenv("STAR_PANEL_TEST_HELPER", "1")
	dir := t.TempDir()
	writeBackendPlugin(t, dir, "backend-demo")

	backends := NewBackends(NewRegistry(dir), nil)
	t.Cleanup(backends.Stop)
	backends.Start()

	handler, err := backends.Handler("backend-demo")
	if err != nil {
		t.Fatalf("handler after Start: %v", err)
	}
	if status, body := getFromHandler(t, handler, "/ping"); status != http.StatusOK {
		t.Fatalf("status = %d, body %s", status, body)
	}
}

func TestHandlerErrorsNameWhatIsWrong(t *testing.T) {
	dir := t.TempDir()
	writePluginFolder(t, dir, "frontend-only", map[string]string{
		"manifest.json": `{"name":"frontend-only","version":"0.1.0","widgets":[{"id":"w","title":"W","module":"widget.js"}]}`,
		"widget.js":     "export default () => {};",
	})
	backends := NewBackends(NewRegistry(dir), nil)
	t.Cleanup(backends.Stop)

	if _, err := backends.Handler("nope"); !errors.Is(err, ErrPluginNotFound) {
		t.Errorf("unknown name error = %v, want ErrPluginNotFound", err)
	}
	_, err := backends.Handler("frontend-only")
	if err == nil || !strings.Contains(err.Error(), "no backend") {
		t.Errorf("frontend-only error = %v, want a no-backend reason", err)
	}
}

func TestHandlerReportsACrashedBackendAsUnavailable(t *testing.T) {
	t.Setenv("STAR_PANEL_TEST_HELPER", "1")
	dir := t.TempDir()
	writeBackendPlugin(t, dir, "crash-demo")

	backends := NewBackends(NewRegistry(dir), nil)
	t.Cleanup(backends.Stop)

	_, err := backends.Handler("crash-demo")
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("crashed backend error = %v, want an unavailable reason", err)
	}
}

// A built-in answers in process: no command, no port, same seam.
func TestHandlerPrefersABuiltIn(t *testing.T) {
	dir := t.TempDir()
	writePluginFolder(t, dir, "system-stats", map[string]string{
		"manifest.json": `{"name":"system-stats","version":"0.1.0","widgets":[{"id":"system-stats","title":"System Stats","module":"widget.js"}]}`,
		"widget.js":     "export default () => {};",
	})
	builtin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "in process: %s", filepath.Base(r.URL.Path))
	})
	backends := NewBackends(NewRegistry(dir), map[string]http.Handler{"system-stats": builtin})
	t.Cleanup(backends.Stop)

	handler, err := backends.Handler("system-stats")
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	status, body := getFromHandler(t, handler, "/stats")
	if status != http.StatusOK || !strings.Contains(body, "in process: stats") {
		t.Errorf("status = %d, body = %s", status, body)
	}
}
