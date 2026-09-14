package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const helloManifest = `{"name":"hello-widget","version":"0.1.0",` +
	`"widgets":[{"id":"hello","title":"Hello","module":"widget.js"}]}`

// zipOf builds the archive a person would upload.
func zipOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return buffer.Bytes()
}

// postArchive hands the server a ZIP the way the panel's import button does.
func postArchive(t *testing.T, ts *httptest.Server, archive []byte) (int, map[string]any) {
	t.Helper()
	res, err := ts.Client().Post(ts.URL+"/api/v1/plugins", "application/zip", bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("post archive: %v", err)
	}
	defer res.Body.Close()
	body := map[string]any{}
	if raw, _ := io.ReadAll(res.Body); len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode %d body %q: %v", res.StatusCode, raw, err)
		}
	}
	return res.StatusCode, body
}

// Import a Plugin, then download it again: the whole trip through the panel,
// in the two shapes the edit mode's buttons use.
func TestPluginImportDownloadAndServe(t *testing.T) {
	ts, pluginsDir := newPluginServer(t, nil)
	defer ts.Close()

	status, body := postArchive(t, ts, zipOf(t, map[string]string{
		"hello-widget/manifest.json": helloManifest,
		"hello-widget/widget.js":     "export default function render(el, ctx) {}",
	}))
	if status != http.StatusCreated {
		t.Fatalf("import status = %d (%v), want 201", status, body)
	}
	if body["name"] != "hello-widget" || body["version"] != "0.1.0" {
		t.Errorf("imported = %v, want the Plugin the archive declares", body)
	}
	if _, problem := body["problem"]; problem {
		t.Errorf("imported = %v, want no problem for a Plugin that needs nothing", body)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "hello-widget", "widget.js")); err != nil {
		t.Errorf("the Plugin did not land in the plugins folder: %v", err)
	}

	// The listing picks it up on its own, and the Widget's module is served
	// from the folder the archive was unpacked into.
	_, list := doJSON(t, ts, http.MethodGet, "/api/v1/plugins", nil)
	plugins, _ := list["plugins"].([]any)
	if len(plugins) != 1 {
		t.Fatalf("plugins = %v, want the imported one", plugins)
	}
	res, err := ts.Client().Get(ts.URL + "/api/v1/plugins/hello-widget/modules/widget.js")
	if err != nil {
		t.Fatalf("get module: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("module status = %d, want 200", res.StatusCode)
	}

	// Download answers with the folder as a ZIP, which is what an import takes.
	download, err := ts.Client().Get(ts.URL + "/api/v1/plugins/hello-widget/archive")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer download.Body.Close()
	if download.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, want 200", download.StatusCode)
	}
	if ct := download.Header.Get("Content-Type"); !strings.Contains(ct, "application/zip") {
		t.Errorf("download content type = %q, want application/zip", ct)
	}
	if disposition := download.Header.Get("Content-Disposition"); !strings.Contains(disposition, "hello-widget.zip") {
		t.Errorf("download disposition = %q, want the Plugin's name as a file name", disposition)
	}
	archive, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("the download is not a ZIP: %v", err)
	}
	names := []string{}
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	if strings.Join(names, ",") != "hello-widget/manifest.json,hello-widget/widget.js" {
		t.Errorf("archive holds %v, want the Plugin's files under its name", names)
	}
}

// A Plugin is refused rather than overwritten, and the message says which
// folder is in the way: deleting it is the owner's move.
func TestPluginImportRefusesANameAlreadyInTheFolder(t *testing.T) {
	ts, _ := newPluginServer(t, map[string]map[string]string{
		"hello-widget": {
			"manifest.json": helloManifest,
			"widget.js":     "export default () => {};",
		},
	})
	defer ts.Close()

	status, body := postArchive(t, ts, zipOf(t, map[string]string{
		"hello-widget/manifest.json": helloManifest,
		"hello-widget/widget.js":     "export default () => {};",
	}))
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	message, _ := body["error"].(string)
	if !strings.Contains(message, "hello-widget") {
		t.Errorf("error = %q, want the folder that is in the way named", message)
	}
}

// A ZIP from a stranger is not a Plugin until the Manifest inside it says so,
// and one that says nothing is rejected without leaving a folder behind.
func TestPluginImportRejectsAnArchiveThatIsNotAPlugin(t *testing.T) {
	ts, pluginsDir := newPluginServer(t, nil)
	defer ts.Close()

	cases := map[string]map[string]string{
		"no manifest":                    {"hello-widget/widget.js": "export default () => {};"},
		"a manifest that does not parse": {"hello-widget/manifest.json": "{not json"},
		"a name that is not a folder name": {
			"hello-widget/manifest.json": `{"name":"../escape","version":"0.1.0",` +
				`"widgets":[{"id":"w","title":"W","module":"widget.js"}]}`,
		},
		"an entry that climbs out of the folder": {
			"hello-widget/manifest.json": helloManifest,
			"../escaped.txt":             "should never be written",
		},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			status, body := postArchive(t, ts, zipOf(t, entries))
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d (%v), want 400", status, body)
			}
			if _, ok := body["error"]; !ok {
				t.Errorf("body = %v, want a reason", body)
			}
			if names, err := os.ReadDir(pluginsDir); err != nil || len(names) != 0 {
				t.Errorf("plugins dir holds %v (%v), want nothing left behind", names, err)
			}
		})
	}
}

// A Plugin that declares something this machine does not have is reported the
// moment it lands, not after somebody adds its Widget.
func TestPluginImportReportsAMissingRequirement(t *testing.T) {
	ts, _ := newPluginServer(t, nil)
	defer ts.Close()

	status, body := postArchive(t, ts, zipOf(t, map[string]string{
		"needs-node/manifest.json": `{"name":"needs-node","version":"0.1.0",` +
			`"widgets":[{"id":"w","title":"W","module":"widget.js"}],` +
			`"requires":[{"command":"definitely-not-installed-xyz","minVersion":"20"}]}`,
		"needs-node/widget.js": "export default () => {};",
	}))
	if status != http.StatusCreated {
		t.Fatalf("status = %d (%v), want 201", status, body)
	}
	problem, _ := body["problem"].(string)
	if !strings.Contains(problem, "definitely-not-installed-xyz") || !strings.Contains(problem, "20") {
		t.Errorf("problem = %q, want what the Plugin needs and the version it asked for", problem)
	}
}

// Downloading a Plugin that is not there is a 404 in JSON, not an empty file.
func TestPluginDownloadOfAnUnknownPlugin(t *testing.T) {
	ts, _ := newPluginServer(t, nil)
	defer ts.Close()

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/plugins/nope/archive", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d (%v), want 404", res.StatusCode, body)
	}
}
