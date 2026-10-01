package plugins

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// zipOfMode builds an archive whose entries carry a Unix mode, which is the
// whole point of a Plugin that ships its own binary: `writer.Create` writes
// 0644 and a compiled backend then arrives unexecutable.
func zipOfMode(t *testing.T, entries map[string]struct {
	body string
	mode fs.FileMode
}) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, entry := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := file.Write([]byte(entry.body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return buffer.Bytes()
}

// A Plugin that brings its own binary is the reason this matters: the archive
// is the only way it travels, and a binary that loses its execute bit on the
// way in is a Plugin that installs, reports healthy, and never runs.
//
// The execute bit is a Unix idea, so the assertion is Unix-only; the mode is
// still written and read on every platform, which is what keeps this file
// compiling and the round trip honest where the test can see it.
func TestImportKeepsAnArchivesExecuteBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an execute bit is not a thing Windows has")
	}
	dir := t.TempDir()
	registry := NewRegistry(dir)

	archive := zipOfMode(t, map[string]struct {
		body string
		mode fs.FileMode
	}{
		"disk-space/manifest.json": {`{"name":"disk-space","version":"0.1.0","widgets":[{"id":"disk-space","title":"Disk","module":"widget.js"}],"backend":{"command":["./backend"]}}`, 0o644},
		"disk-space/widget.js":     {"export default () => {}", 0o644},
		"disk-space/backend":       {"#!/bin/sh\n", 0o755},
	})

	if _, err := registry.Import(archive); err != nil {
		t.Fatalf("import: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "disk-space", "backend"))
	if err != nil {
		t.Fatalf("stat the unpacked binary: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("the unpacked binary is not executable: mode %v", info.Mode().Perm())
	}
}

// An archive is a folder from a stranger, so it may grant execute and nothing
// more: setuid, setgid and a world-writable file are not things a Plugin gets
// to hand the panel.
func TestImportNeverGrantsMoreThanExecute(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports synthetic permissions, so the mode is not readable back")
	}
	dir := t.TempDir()
	registry := NewRegistry(dir)

	archive := zipOfMode(t, map[string]struct {
		body string
		mode fs.FileMode
	}{
		"loud/manifest.json":       {`{"name":"loud","version":"0.1.0","widgets":[{"id":"loud","title":"Loud","module":"widget.js"}]}`, 0o644},
		"loud/widget.js":           {"export default () => {}", 0o644},
		"loud/setuid-thing":        {"x", 0o4755},
		"loud/world-writable-file": {"x", 0o666},
	})

	if _, err := registry.Import(archive); err != nil {
		t.Fatalf("import: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "loud", "setuid-thing"))
	if err != nil {
		t.Fatalf("stat setuid-thing: %v", err)
	}
	if info.Mode()&os.ModeSetuid != 0 {
		t.Errorf("the import granted setuid: mode %v", info.Mode())
	}
	if info.Mode().Perm()&0o002 != 0 {
		t.Errorf("the import granted a world-writable file: mode %v", info.Mode().Perm())
	}
}

// A Download is the other half of the same promise: an archive that cannot be
// imported back into a working Plugin is worse than no download at all.
func TestArchiveCarriesTheExecuteBitBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an execute bit is not a thing Windows has")
	}
	dir := t.TempDir()
	folder := filepath.Join(dir, "disk-space")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(folder, "manifest.json"),
		`{"name":"disk-space","version":"0.1.0","widgets":[{"id":"disk-space","title":"Disk","module":"widget.js"}],"backend":{"command":["./backend"]}}`)
	writeFile(t, filepath.Join(folder, "widget.js"), "export default () => {}")
	if err := os.WriteFile(filepath.Join(folder, "backend"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write the binary: %v", err)
	}

	entry := Entry{Folder: "disk-space", Dir: folder}
	archive, err := entry.Archive()
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("read the archive back: %v", err)
	}
	var found bool
	for _, file := range reader.File {
		if !strings.HasSuffix(file.Name, "/backend") {
			continue
		}
		found = true
		if file.Mode().Perm()&0o111 == 0 {
			t.Errorf("the archive lost the execute bit: mode %v", file.Mode().Perm())
		}
	}
	if !found {
		t.Fatal("the archive holds no backend entry")
	}
}

// And the round trip end to end, which is the only test that proves both
// halves agree.
func TestArchiveRoundTripKeepsTheBinaryRunnable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an execute bit is not a thing Windows has")
	}
	source := t.TempDir()
	folder := filepath.Join(source, "disk-space")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(folder, "manifest.json"),
		`{"name":"disk-space","version":"0.1.0","widgets":[{"id":"disk-space","title":"Disk","module":"widget.js"}],"backend":{"command":["./backend"]}}`)
	writeFile(t, filepath.Join(folder, "widget.js"), "export default () => {}")
	if err := os.WriteFile(filepath.Join(folder, "backend"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write the binary: %v", err)
	}

	archive, err := Entry{Folder: "disk-space", Dir: folder}.Archive()
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	destination := t.TempDir()
	if _, err := NewRegistry(destination).Import(archive); err != nil {
		t.Fatalf("import the download: %v", err)
	}
	info, err := os.Stat(filepath.Join(destination, "disk-space", "backend"))
	if err != nil {
		t.Fatalf("stat the re-imported binary: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("the binary did not survive the round trip: mode %v", info.Mode().Perm())
	}
}

// A Plugin whose own binary is not executable is a broken Plugin, and the
// listing has to say so: core would otherwise log "permission denied" on a
// restart loop while the Plugins section looked fine.
func TestLoadManifestReportsANonExecutableBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an execute bit is not a thing Windows has")
	}
	folder := filepath.Join(t.TempDir(), "broken")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(folder, "manifest.json"),
		`{"name":"broken","version":"0.1.0","widgets":[{"id":"broken","title":"Broken","module":"widget.js"}],"backend":{"command":["./backend"]}}`)
	if err := os.WriteFile(filepath.Join(folder, "backend"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write the binary: %v", err)
	}

	_, err := LoadManifest(folder)
	if err == nil {
		t.Fatal("a non-executable backend was not reported")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
}

// The check is about the Plugin's own binary, not about the tools the machine
// is supposed to provide: a bare name is looked for in PATH, where a bit is
// not ours to check.
func TestLoadManifestAcceptsABareCommandName(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "fine")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(folder, "manifest.json"),
		`{"name":"fine","version":"0.1.0","widgets":[{"id":"fine","title":"Fine","module":"widget.js"}],"backend":{"command":["node","backend.mjs"]}}`)

	if _, err := LoadManifest(folder); err != nil {
		t.Fatalf("a bare command name was refused: %v", err)
	}
}

// A panel whose plugins folder has never held anything has no plugins folder
// yet. Discovery treats that as "no Plugins", so an import must too, rather
// than failing on the staging folder it is about to create inside.
func TestImportCreatesAPluginsFolderThatIsNotThereYet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plugins-that-does-not-exist")
	registry := NewRegistry(dir)

	archive := zipOf(t, map[string]string{
		"fresh/manifest.json": `{"name":"fresh","version":"0.1.0","widgets":[{"id":"fresh","title":"Fresh","module":"widget.js"}]}`,
		"fresh/widget.js":     "export default () => {}",
	})

	if _, err := registry.Import(archive); err != nil {
		t.Fatalf("import into a folder that does not exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fresh", "manifest.json")); err != nil {
		t.Fatalf("the Plugin was not unpacked: %v", err)
	}
}

// The mode an entry is unpacked with is a pure decision, so it is tested on
// every platform — the Unix tests above have to be skipped on Windows because
// the mode cannot be read back there, and this is the part that must not
// silently regress.
func TestUnpackedModeGrantsExecuteAndNothingMore(t *testing.T) {
	cases := []struct {
		name string
		from os.FileMode
		want os.FileMode
	}{
		{"a plain file stays readable", 0o644, 0o644},
		{"an executable keeps its bit", 0o755, 0o755},
		{"owner-only execute still counts", 0o744, 0o744},
		{"no mode at all is readable", 0, 0o644},
		{"setuid is dropped", os.FileMode(0o4755), 0o755},
		{"setgid is dropped", os.FileMode(0o2755), 0o755},
		{"sticky is dropped", os.FileMode(0o1755), 0o755},
		{"world-writable is dropped", 0o666, 0o644},
		{"group-writable is dropped", 0o664, 0o644},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := unpackedMode(c.from)
			if got != c.want {
				t.Errorf("unpackedMode(%v) = %v, want %v", c.from, got, c.want)
			}
			if got&os.ModeSetuid != 0 || got&os.ModeSetgid != 0 {
				t.Errorf("unpackedMode(%v) granted a setuid or setgid bit: %v", c.from, got)
			}
			if got.Perm()&0o002 != 0 {
				t.Errorf("unpackedMode(%v) granted a world-writable file: %v", c.from, got.Perm())
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
