package plugins

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Registry manages plugin discovery from the filesystem on demand.
type Registry struct {
	dir string
}

func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir}
}

// Entry represents a discovered plugin directory and its status.
type Entry struct {
	Folder   string
	Dir      string
	Manifest Manifest
	Err      error
}

// Missing returns declared requirements that are not found in PATH or the plugin folder.
func (e Entry) Missing() []Requirement {
	var missing []Requirement
	for _, required := range e.Manifest.Requires {
		if !reachable(required.Command, e.Dir) {
			missing = append(missing, required)
		}
	}
	return missing
}

func reachable(command, dir string) bool {
	if strings.ContainsAny(command, `/\`) {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(command)))
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(command)
	return err == nil
}

// Discover scans the plugins directory and loads all entries.
func (r *Registry) Discover() []Entry {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("scan plugins dir: %v", err)
		}
		return nil
	}
	var found []Entry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(r.dir, e.Name())
		manifest, err := LoadManifest(dir)
		found = append(found, Entry{
			Folder:   e.Name(),
			Dir:      dir,
			Manifest: manifest,
			Err:      err,
		})
	}
	return found
}

// Find returns the entry matching the given folder name.
func (r *Registry) Find(name string) (Entry, bool) {
	for _, entry := range r.Discover() {
		if entry.Folder == name {
			return entry, true
		}
	}
	return Entry{}, false
}
