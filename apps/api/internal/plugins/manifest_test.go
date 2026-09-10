package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePlugin(t *testing.T, folder string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), folder)
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

const validManifest = `{
  "name": "hello-widget",
  "version": "0.1.0",
  "widgets": [{"id": "hello", "title": "Hello", "module": "widget.js"}]
}`

func TestLoadManifestAcceptsValidPlugin(t *testing.T) {
	dir := writePlugin(t, "hello-widget", map[string]string{
		"manifest.json": validManifest,
		"widget.js":     "export default () => {};",
	})
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "hello-widget" || m.Version != "0.1.0" {
		t.Errorf("unexpected manifest: %+v", m)
	}
	if len(m.Widgets) != 1 || m.Widgets[0].ID != "hello" || m.Widgets[0].Module != "widget.js" {
		t.Errorf("widgets not parsed: %+v", m.Widgets)
	}
	if m.Backend != nil {
		t.Errorf("frontend-only plugin parsed a backend: %+v", m.Backend)
	}
}

func TestLoadManifestRejectsInvalidPlugins(t *testing.T) {
	cases := map[string]map[string]string{
		"missing manifest": {},
		"malformed json":   {"manifest.json": "{oops"},
		"empty name":       {"manifest.json": `{"name":"","version":"0.1.0","widgets":[{"id":"a","module":"w.js"}]}`, "w.js": ""},
		"folder mismatch": {
			"manifest.json": `{"name":"other-name","version":"0.1.0","widgets":[{"id":"a","module":"w.js"}]}`,
			"w.js":          "",
		},
		"bad name characters": {
			"manifest.json": `{"name":"Hello Widget","version":"0.1.0","widgets":[{"id":"a","module":"w.js"}]}`,
			"w.js":          "",
		},
		"missing version": {"manifest.json": `{"name":"p","widgets":[{"id":"a","module":"w.js"}]}`, "w.js": ""},
		"no widgets":      {"manifest.json": `{"name":"p","version":"0.1.0","widgets":[]}`, "w.js": ""},
		"duplicate widget ids": {
			"manifest.json": `{"name":"p","version":"0.1.0","widgets":[{"id":"a","module":"w.js"},{"id":"a","module":"w2.js"}]}`,
			"w.js":          "", "w2.js": "",
		},
		"widget missing module": {"manifest.json": `{"name":"p","version":"0.1.0","widgets":[{"id":"a","module":""}]}`, "w.js": ""},
		"backend without command": {
			"manifest.json": `{"name":"p","version":"0.1.0","widgets":[{"id":"a","module":"w.js"}],"backend":{"command":[]}}`,
			"w.js":          "",
		},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			folder := "p"
			if strings.Contains(name, "folder mismatch") {
				folder = "hello-widget"
			}
			if _, err := LoadManifest(writePlugin(t, folder, files)); err == nil {
				t.Fatalf("expected error for %q, got none", name)
			}
		})
	}
}

func TestModuleFileRejectsEscapePaths(t *testing.T) {
	dir := writePlugin(t, "hello-widget", map[string]string{
		"manifest.json": validManifest,
		"widget.js":     "export default () => {};",
	})
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	for _, bad := range []string{"../secret.txt", "..\\secret.txt", "a/../../secret.txt", "a\\..\\..\\secret.txt"} {
		if got, err := m.ModuleFile(dir, bad); err == nil {
			t.Errorf("ModuleFile(%q) = %q, want escape rejection", bad, got)
		}
	}
	got, err := m.ModuleFile(dir, "widget.js")
	if err != nil {
		t.Fatalf("ModuleFile(widget.js): %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("hello-widget", "widget.js")) {
		t.Errorf("ModuleFile(widget.js) = %q", got)
	}
}
