package plugins

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Registry discovers Plugins by folder drop under the plugins directory. Every
// call re-reads the folder, so dropping one in is picked up by the next
// request rather than needing a restart.
type Registry struct {
	dir string
}

func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir}
}

// Entry is one discovered folder: either a valid Plugin or the reason it was
// rejected. An invalid folder never breaks the listing; the owner needs to
// know why their Plugin is missing from the panel.
type Entry struct {
	Folder   string
	Dir      string
	Manifest Manifest
	Err      error
}

// Missing reports the requirements this Plugin declares that are not reachable
// from where it sits, so the panel can say what is wrong before a Widget fails
// with a timeout. A bare name is looked up in the PATH; a path with a separator
// in it — ./backend — is looked for inside the Plugin's own folder, which is
// where its working directory points.
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

// Discover scans the plugins directory, oldest name first as the filesystem
// reports it.
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

// Find returns the Entry for one folder name.
func (r *Registry) Find(name string) (Entry, bool) {
	for _, entry := range r.Discover() {
		if entry.Folder == name {
			return entry, true
		}
	}
	return Entry{}, false
}
