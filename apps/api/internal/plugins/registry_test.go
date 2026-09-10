package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func writePluginFolder(t *testing.T, dir, name string, files map[string]string) {
	t.Helper()
	folder := filepath.Join(dir, name)
	for file, content := range files {
		path := filepath.Join(folder, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", file, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
}

const goodManifest = `{"name":"good","version":"0.1.0","widgets":[{"id":"w","title":"W","module":"widget.js"}]}`

// A folder that does not parse has to come back as an Entry with a reason:
// silently skipping it would leave the owner wondering where their Plugin went.
func TestDiscoverReportsRejectedFolders(t *testing.T) {
	dir := t.TempDir()
	writePluginFolder(t, dir, "good", map[string]string{
		"manifest.json": goodManifest,
		"widget.js":     "export default () => {};",
	})
	writePluginFolder(t, dir, "broken", map[string]string{"manifest.json": "{not json"})
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}

	entries := NewRegistry(dir).Discover()
	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want the two visible folders", entries)
	}
	byFolder := map[string]Entry{}
	for _, entry := range entries {
		byFolder[entry.Folder] = entry
	}
	if byFolder["good"].Err != nil {
		t.Errorf("valid folder reported %v", byFolder["good"].Err)
	}
	if byFolder["broken"].Err == nil {
		t.Error("broken folder was accepted without a reason")
	}
}

func TestFindAndModuleFile(t *testing.T) {
	dir := t.TempDir()
	writePluginFolder(t, dir, "good", map[string]string{
		"manifest.json":  goodManifest,
		"widget.js":      "export default () => {};",
		"nested/util.js": "export const x = 1;",
	})
	registry := NewRegistry(dir)

	entry, ok := registry.Find("good")
	if !ok {
		t.Fatal("Find missed a folder that exists")
	}
	if _, err := entry.Manifest.ModuleFile(entry.Dir, "widget.js"); err != nil {
		t.Errorf("own module rejected: %v", err)
	}
	if _, err := entry.Manifest.ModuleFile(entry.Dir, "nested/util.js"); err != nil {
		t.Errorf("nested module rejected: %v", err)
	}
	for _, escape := range []string{"../secret.js", "nested/../../secret.js"} {
		if _, err := entry.Manifest.ModuleFile(entry.Dir, escape); err == nil {
			t.Errorf("module path %q escaped the Plugin folder", escape)
		}
	}
	if _, ok := registry.Find("nope"); ok {
		t.Error("Find invented an entry for a folder that does not exist")
	}
}
