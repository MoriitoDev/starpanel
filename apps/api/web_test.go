package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// The embedded build is only as good as its routing rules, and those rules
// must hold whatever the build happens to contain, so these tests drive the
// handler with their own filesystem instead of the compiled-in one.
func testDashboardFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {Data: []byte(
			`<!doctype html><html><head><title>Star Panel</title></head>` +
				`<body><div id="app"></div></body></html>`,
		)},
		"assets/app.js": {Data: []byte("console.log('panel')")},
		"favicon.svg":   {Data: []byte("<svg/>")},
	}
}

func getFromWeb(t *testing.T, root fs.FS, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	webHandler(root, noTheme).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// noTheme is the shell's view of a Dashboard that renders with the baseline.
func noTheme() string { return "" }

func TestWebHandlerServesTheShellAtTheRoot(t *testing.T) {
	res := getFromWeb(t, testDashboardFS(), "/")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Star Panel") {
		t.Errorf("body = %q, want the app shell", res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("content type = %q, want text/html", got)
	}
}

func TestWebHandlerServesBuiltAssets(t *testing.T) {
	res := getFromWeb(t, testDashboardFS(), "/assets/app.js")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if !strings.Contains(res.Body.String(), "console.log") {
		t.Errorf("body = %q, want the module", res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.Contains(got, "javascript") {
		t.Errorf("content type = %q, want javascript", got)
	}
}

// The SPA has no routes today, but a path without a file extension must still
// land on the shell rather than a bare 404 if one is ever added.
func TestWebHandlerFallsBackToTheShellForUnroutedPaths(t *testing.T) {
	res := getFromWeb(t, testDashboardFS(), "/settings")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Star Panel") {
		t.Errorf("body = %q, want the app shell", res.Body.String())
	}
}

// A missing asset must not answer with HTML: the browser would try to parse
// the shell as a module and fail with a confusing error.
func TestWebHandlerDoesNotServeTheShellForMissingAssets(t *testing.T) {
	res := getFromWeb(t, testDashboardFS(), "/assets/stale-hash.js")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
	if strings.Contains(res.Body.String(), "Star Panel") {
		t.Errorf("body = %q, want no app shell", res.Body.String())
	}
}

// An unrouted API path is a JSON client's mistake, not a page request.
func TestWebHandlerKeepsUnroutedAPIPathsAsJSON(t *testing.T) {
	res := getFromWeb(t, testDashboardFS(), "/api/v1/nope")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if !strings.Contains(res.Body.String(), "no such route") {
		t.Errorf("body = %q, want a JSON error", res.Body.String())
	}
}

// A checkout that never built the front still compiles and runs; it has to
// say what is missing instead of serving an empty page.
func TestWebHandlerExplainsAMissingBuild(t *testing.T) {
	empty := fstest.MapFS{}
	res := getFromWeb(t, empty, "/")
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res.Code)
	}
	if !strings.Contains(res.Body.String(), "pnpm build") {
		t.Errorf("body = %q, want the build instruction", res.Body.String())
	}
}
