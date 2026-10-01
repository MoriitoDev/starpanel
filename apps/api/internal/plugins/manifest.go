package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Manifest is a plugin's declared name, version, widgets, and backend
// contract (CONTEXT.md). It lives as manifest.json inside the plugin
// folder, and the folder name must match Name.
type Manifest struct {
	Name     string           `json:"name"`
	Version  string           `json:"version"`
	Widgets  []ManifestWidget `json:"widgets"`
	Backend  *BackendSpec     `json:"backend,omitempty"`
	Requires []Requirement    `json:"requires,omitempty"`
}

// Requirement is something a Plugin needs on the machine to run: a command that
// has to be reachable, and the version its author had in mind.
//
// The version is shown, never judged. Comparing versions means parsing them and
// every tool writes them differently; an owner reading "needs node >= 20" next
// to the version they have is a better answer than a guess.
type Requirement struct {
	Command    string `json:"command"`
	MinVersion string `json:"minVersion,omitempty"`
}

// ManifestWidget declares one widget the plugin provides. Module is the
// plugin-folder-relative path of the framework-free ESM entry.
type ManifestWidget struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Module string `json:"module"`
}

// BackendSpec declares the optional supervised subprocess. {port} in any
// argument is replaced with the allocated localhost port; it is also
// exported as the STAR_PANEL_PORT env var.
type BackendSpec struct {
	Command []string `json:"command"`
}

// LoadManifest reads and validates manifest.json from a plugin folder.
// The folder argument is the plugin's identity: Name must match it.
func LoadManifest(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.validate(filepath.Base(dir)); err != nil {
		return Manifest{}, err
	}
	if err := m.executableBackend(dir); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// executableBackend catches the one failure that otherwise costs an hour: a
// Plugin whose own binary arrived without its execute bit. Core would log
// `backend exited (permission denied); restarting in 2s` on a loop and the
// Plugins section would look healthy, because `reachable` asks os.Stat and a
// stat says the file is there.
//
// Two things are deliberately out of scope. A bare name is looked up in PATH,
// where the bit belongs to the machine and is not ours to judge. And on Windows
// the bit does not exist — chmod there only moves the read-only flag — so the
// whole check is skipped rather than refusing every Plugin on a platform whose
// modes mean something else. The Plugin is built for linux/amd64, which is
// where this question has an answer.
func (m *Manifest) executableBackend(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if m.Backend == nil || len(m.Backend.Command) == 0 {
		return nil
	}
	name := m.Backend.Command[0]
	if !strings.ContainsAny(name, `/\`) {
		return nil
	}
	file := filepath.Join(dir, filepath.FromSlash(name))
	info, err := os.Stat(file)
	if err != nil {
		// A missing file is the existing story: the backend will not start and
		// core says so in the log. Reporting it here would turn a Plugin with a
		// typo in one argument into a Plugin the panel refuses to list.
		return nil
	}
	if info.IsDir() {
		return fmt.Errorf("manifest: backend %q is a directory", name)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf(
			"manifest: backend %q is not executable — the archive it arrived in lost its mode, so rebuild it with build.ps1 or chmod +x it", name)
	}
	return nil
}

func (m *Manifest) validate(folderName string) error {
	if err := validName(m.Name); err != nil {
		return err
	}
	if m.Name != folderName {
		return fmt.Errorf("manifest: name %q must match folder name %q", m.Name, folderName)
	}
	if m.Version == "" {
		return errors.New("manifest: version is required")
	}
	if len(m.Widgets) == 0 {
		return errors.New("manifest: at least one widget is required")
	}
	seen := map[string]bool{}
	for i, w := range m.Widgets {
		if w.ID == "" {
			return fmt.Errorf("manifest: widget %d: id is required", i)
		}
		if !namePattern.MatchString(w.ID) {
			return fmt.Errorf("manifest: widget %q: id must be lowercase letters, digits, and dashes", w.ID)
		}
		if seen[w.ID] {
			return fmt.Errorf("manifest: widget %q: duplicate id", w.ID)
		}
		seen[w.ID] = true
		if w.Module == "" {
			return fmt.Errorf("manifest: widget %q: module is required", w.ID)
		}
	}
	if m.Backend != nil && len(m.Backend.Command) == 0 {
		return errors.New("manifest: backend command is required when backend is declared")
	}
	for i, required := range m.Requires {
		if required.Command == "" {
			return fmt.Errorf("manifest: requires %d: command is required", i)
		}
	}
	return nil
}

// validName is the rule for the name a Plugin goes by, which is also the name
// of its folder. Import applies it before the name reaches the filesystem; a
// dropped-in folder meets it in validate.
func validName(name string) error {
	if name == "" {
		return errors.New("manifest: name is required")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("manifest: name %q must be lowercase letters, digits, and dashes", name)
	}
	return nil
}

// ModuleFile resolves a widget's module path inside the plugin folder,
// rejecting paths that escape it.
func (m *Manifest) ModuleFile(dir string, modulePath string) (string, error) {
	file, ok := insideFolder(dir, modulePath)
	if !ok {
		return "", fmt.Errorf("module path %q escapes the plugin folder", modulePath)
	}
	return file, nil
}

// insideFolder resolves a folder-relative path and reports whether it landed
// inside. Both a widget's module path and an archive entry come through here:
// a ZIP is a folder from a stranger, and this is the one guard that decides
// where a stranger's path may point.
func insideFolder(folder, relative string) (string, bool) {
	full := filepath.Join(folder, filepath.FromSlash(relative))
	within, err := filepath.Rel(folder, full)
	if err != nil || filepath.IsAbs(within) || hasParentPrefix(within) {
		return "", false
	}
	return full, true
}

// hasParentPrefix reports whether a relative path starts by climbing out of
// the folder it is measured from. It is handed slash-separated paths on every
// platform, so a Windows build catches what a Linux one would.
func hasParentPrefix(rel string) bool {
	rel = filepath.ToSlash(rel)
	return rel == ".." || strings.HasPrefix(rel, "../")
}
