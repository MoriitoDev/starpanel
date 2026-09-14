package plugins

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// zipOf builds the archive a person would hand the panel: names are the paths
// inside it, slashes and all.
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

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

// Import is how a Plugin arrives without anyone copying a folder by hand: the
// archive is unpacked into the folder the Registry already reads.
func TestImportUnpacksAnArchiveIntoThePluginsFolder(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry(dir)

	entry, err := registry.Import(zipOf(t, map[string]string{
		"good/manifest.json": goodManifest,
		"good/widget.js":     "export default () => {};",
	}))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if entry.Folder != "good" || entry.Err != nil {
		t.Errorf("entry = %+v, want the good folder with no error", entry)
	}
	if _, err := os.Stat(filepath.Join(dir, "good", "widget.js")); err != nil {
		t.Errorf("widget module missing after import: %v", err)
	}
	// Nothing is left behind: the archive is never a file on disk, and neither
	// is the folder it was unpacked into before it was moved into place.
	if names := dirNames(t, dir); len(names) != 1 || names[0] != "good" {
		t.Errorf("plugins dir holds %v, want just the plugin folder", names)
	}
	if _, ok := registry.Find("good"); !ok {
		t.Error("the Registry does not see the imported folder")
	}
}

// A Plugin is somebody's work, and its backend is a program core would run, so
// a name that is already there is refused — and the refusal names the folder,
// because deleting the old one is the owner's move.
func TestImportRefusesANameThatIsAlreadyThere(t *testing.T) {
	dir := t.TempDir()
	writePluginFolder(t, dir, "good", map[string]string{
		"manifest.json": goodManifest,
		"widget.js":     "export default function render() { return original; }",
	})
	registry := NewRegistry(dir)

	_, err := registry.Import(zipOf(t, map[string]string{
		"good/manifest.json": goodManifest,
		"good/widget.js":     "export default function render() { return replaced; }",
	}))
	if !errors.Is(err, ErrExists) {
		t.Fatalf("import error = %v, want ErrExists", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "good")) {
		t.Errorf("error = %q, want the folder it collided with named", err)
	}
	kept, err := os.ReadFile(filepath.Join(dir, "good", "widget.js"))
	if err != nil {
		t.Fatalf("read the existing plugin: %v", err)
	}
	if !strings.Contains(string(kept), "original") {
		t.Errorf("existing widget.js = %q, want the folder left as it was", kept)
	}
}

// A broken upload leaves nothing behind: not a half-written Plugin, not a
// stray file outside the folder, and not a staging folder to tidy up later.
func TestImportLeavesNothingBehindWhenTheArchiveIsNotAPlugin(t *testing.T) {
	cases := map[string]map[string]string{
		"no manifest at all": {
			"good/widget.js": "export default () => {};",
		},
		"manifest that does not parse": {
			"good/manifest.json": "{not json",
		},
		"manifest with no widgets": {
			"good/manifest.json": `{"name":"good","version":"0.1.0"}`,
		},
		"manifest with a widget but no module": {
			"good/manifest.json": `{"name":"good","version":"0.1.0","widgets":[{"id":"w","title":"W"}]}`,
		},
		"a name that is not a folder name": {
			"good/manifest.json": `{"name":"../escape","version":"0.1.0",` +
				`"widgets":[{"id":"w","title":"W","module":"widget.js"}]}`,
		},
		"an entry that climbs out of the folder": {
			"good/manifest.json": goodManifest,
			"good/widget.js":     "export default () => {};",
			"../escaped.txt":     "should never be written",
		},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writePluginFolder(t, dir, "already-here", map[string]string{"manifest.json": goodManifest})

			if _, err := NewRegistry(dir).Import(zipOf(t, entries)); err == nil {
				t.Fatal("a broken archive was accepted")
			}
			if names := dirNames(t, dir); len(names) != 1 || names[0] != "already-here" {
				t.Errorf("plugins dir holds %v, want the folder that was already there", names)
			}
			if _, err := os.Stat(filepath.Join(dir, "good")); err == nil {
				t.Error("the rejected Plugin was left in the plugins folder")
			}
			if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); err == nil {
				t.Error("an archive entry was written outside its folder")
			}
		})
	}
}

// An archive made by zipping the contents of a folder has no folder inside it;
// a person means the same Plugin either way.
func TestImportTakesAnArchiveRootedAtTheManifest(t *testing.T) {
	registry := NewRegistry(t.TempDir())

	entry, err := registry.Import(zipOf(t, map[string]string{
		"manifest.json": goodManifest,
		"widget.js":     "export default () => {};",
	}))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if entry.Folder != "good" {
		t.Errorf("folder = %q, want the name the Manifest declares", entry.Folder)
	}
}

// Downloading a Plugin is the other half of importing one: what comes out of
// Archive is a Plugin the same panel would take back in.
func TestArchiveRoundTripsAPluginFolder(t *testing.T) {
	dir := t.TempDir()
	writePluginFolder(t, dir, "good", map[string]string{
		"manifest.json":  goodManifest,
		"widget.js":      "export default () => {};",
		"nested/util.js": "export const x = 1;",
	})
	entry, ok := NewRegistry(dir).Find("good")
	if !ok {
		t.Fatal("the folder to archive was not found")
	}

	archive, err := entry.Archive()
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("read the archive back: %v", err)
	}
	names := []string{}
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	want := []string{"good/manifest.json", "good/nested/util.js", "good/widget.js"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("archive holds %v, want %v", names, want)
	}

	// The round trip is the point: into a fresh plugins folder, under the same
	// name, with the files unchanged.
	elsewhere := t.TempDir()
	if _, err := NewRegistry(elsewhere).Import(archive); err != nil {
		t.Fatalf("import the archive back: %v", err)
	}
	for _, file := range want[1:] {
		original, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		copied, err := os.ReadFile(filepath.Join(elsewhere, filepath.FromSlash(file)))
		if err != nil {
			t.Fatalf("read %s back: %v", file, err)
		}
		if string(copied) != string(original) {
			t.Errorf("%s came back as %q, want %q", file, copied, original)
		}
	}
}
