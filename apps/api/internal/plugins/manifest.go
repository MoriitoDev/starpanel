package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Manifest is a plugin's declared name, version, widgets, and backend
// contract (CONTEXT.md). It lives as manifest.json inside the plugin
// folder, and the folder name must match Name.
type Manifest struct {
	Name    string           `json:"name"`
	Version string           `json:"version"`
	Widgets []ManifestWidget `json:"widgets"`
	Backend *BackendSpec     `json:"backend,omitempty"`
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
	return m, nil
}

func (m *Manifest) validate(folderName string) error {
	if m.Name == "" {
		return errors.New("manifest: name is required")
	}
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("manifest: name %q must be lowercase letters, digits, and dashes", m.Name)
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
	return nil
}

// ModuleFile resolves a widget's module path inside the plugin folder,
// rejecting paths that escape it.
func (m *Manifest) ModuleFile(dir string, modulePath string) (string, error) {
	full := filepath.Join(dir, filepath.FromSlash(modulePath))
	rel, err := filepath.Rel(dir, full)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || hasParentPrefix(rel) {
		return "", fmt.Errorf("module path %q escapes the plugin folder", modulePath)
	}
	return full, nil
}

func hasParentPrefix(rel string) bool {
	return rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}
