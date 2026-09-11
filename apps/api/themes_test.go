package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"star-panel/internal/dashboard"
	"star-panel/internal/themes"
)

const draculaCSS = "/* @name: Dracula Nights */\n:root { --canvas: #282a36; --ink: #f8f8f2; }\n"

// newThemeStore gives a test its own themes folder, the way core gives the
// panel one beside the binary.
func newThemeStore(t *testing.T) *themes.Store {
	t.Helper()
	store, err := themes.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new themes: %v", err)
	}
	return store
}

func dashboardFor(theme string) dashboard.Dashboard {
	return dashboard.Dashboard{Theme: dashboard.ThemeName(theme)}
}

// The default Theme is the baseline inside the binary, not a file, so it has
// to be on the list whatever the folder holds — and it is what a fresh
// Dashboard renders with.
func TestThemeListingAlwaysOffersTheDefault(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/themes", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if active, _ := body["active"].(string); active != "default" {
		t.Errorf("active = %v, want the default", body["active"])
	}
	themes, _ := body["themes"].([]any)
	if len(themes) != 1 {
		t.Fatalf("themes = %v, want just the default", themes)
	}
	first, _ := themes[0].(map[string]any)
	if first["slug"] != "default" || first["present"] != true {
		t.Errorf("first entry = %v, want the default present", first)
	}
}

func postTheme(t *testing.T, ts *httptest.Server, css string) (int, map[string]any) {
	t.Helper()
	res, err := ts.Client().Post(ts.URL+"/api/v1/themes", "text/css", strings.NewReader(css))
	if err != nil {
		t.Fatalf("post theme: %v", err)
	}
	defer res.Body.Close()
	body := map[string]any{}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	return res.StatusCode, body
}

// Import, list, serve, delete: the whole trip a Theme makes through the panel.
func TestThemeImportServeAndDelete(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	status, body := postTheme(t, ts, draculaCSS)
	if status != http.StatusCreated {
		t.Fatalf("import status = %d (%v), want 201", status, body)
	}
	if slug, _ := body["slug"].(string); slug != "dracula-nights" {
		t.Errorf("imported slug = %v, want dracula-nights", body["slug"])
	}

	_, list := doJSON(t, ts, http.MethodGet, "/api/v1/themes", nil)
	themes, _ := list["themes"].([]any)
	if len(themes) != 2 {
		t.Fatalf("themes = %v, want the default and the import", themes)
	}

	res, err := ts.Client().Get(ts.URL + "/api/v1/themes/dracula-nights.css")
	if err != nil {
		t.Fatalf("get theme: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("serve status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("content type = %q, want text/css", ct)
	}
	served, _ := io.ReadAll(res.Body)
	if string(served) != draculaCSS {
		t.Errorf("served %q, want the file back unchanged", served)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/themes/dracula-nights", nil)
	del, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", del.StatusCode)
	}
	again, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/themes/dracula-nights", nil)
	gone, err := ts.Client().Do(again)
	if err != nil {
		t.Fatalf("delete again: %v", err)
	}
	gone.Body.Close()
	if gone.StatusCode != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", gone.StatusCode)
	}
}

// A theme is somebody's work: the panel asks for another name instead of
// overwriting what is already there, and the baseline is not deletable.
func TestThemeImportRefusesAClashAndDeletingTheDefault(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	if status, _ := postTheme(t, ts, draculaCSS); status != http.StatusCreated {
		t.Fatalf("first import status = %d, want 201", status)
	}
	if status, _ := postTheme(t, ts, draculaCSS); status != http.StatusConflict {
		t.Errorf("second import status = %d, want 409", status)
	}
	if status, _ := postTheme(t, ts, "   \n"); status != http.StatusBadRequest {
		t.Errorf("empty import status = %d, want 400", status)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/themes/default", nil)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("delete default: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("deleting the default status = %d, want 400", res.StatusCode)
	}

	if res, err := ts.Client().Get(ts.URL + "/api/v1/themes/nope.css"); err == nil {
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("unknown theme status = %d, want 404", res.StatusCode)
		}
	}
}

// The shell arrives with the active Theme already linked, which is what keeps
// the panel from painting the default and flipping a moment later.
func TestShellLinksTheActiveTheme(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", dashboardFor("dracula-nights"))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d (%v)", res.StatusCode, body)
	}
	if status, _ := postTheme(t, ts, draculaCSS); status != http.StatusCreated {
		t.Fatalf("import status = %d, want 201", status)
	}

	shell, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("get shell: %v", err)
	}
	defer shell.Body.Close()
	html, _ := io.ReadAll(shell.Body)
	if !strings.Contains(string(html), `/api/v1/themes/dracula-nights.css`) {
		t.Errorf("shell does not link the active theme:\n%s", html)
	}

	// The default has no file to link, and a name with nothing behind it must
	// leave the panel on its baseline rather than break it.
	if res, _ := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", dashboardFor("default")); res.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d", res.StatusCode)
	}
	shell2, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("get shell: %v", err)
	}
	defer shell2.Body.Close()
	html2, _ := io.ReadAll(shell2.Body)
	if strings.Contains(string(html2), "/api/v1/themes/") {
		t.Errorf("the default shell should link no theme:\n%s", html2)
	}
}

// A name the folder cannot resolve is reported, not hidden: the owner needs to
// know their Theme is gone.
func TestThemeListingReportsAMissingActiveTheme(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	if res, _ := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", dashboardFor("ghost")); res.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d", res.StatusCode)
	}
	_, body := doJSON(t, ts, http.MethodGet, "/api/v1/themes", nil)
	themes, _ := body["themes"].([]any)
	for _, entry := range themes {
		theme, _ := entry.(map[string]any)
		if theme["slug"] == "ghost" {
			if theme["present"] != false {
				t.Errorf("ghost entry = %v, want present false", theme)
			}
			return
		}
	}
	t.Errorf("the active theme is missing from the list: %v", themes)
}
