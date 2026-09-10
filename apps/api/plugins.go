package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// PluginRegistry discovers plugins by folder drop under the plugins dir.
type PluginRegistry struct {
	pluginsDir string
}

func NewPluginRegistry(pluginsDir string) *PluginRegistry {
	return &PluginRegistry{pluginsDir: pluginsDir}
}

// PluginEntry is one discovered folder: either a valid plugin or a
// validation error. Invalid folders never break the listing.
type PluginEntry struct {
	Folder   string
	Dir      string
	Manifest Manifest
	Err      error
}

func (r *PluginRegistry) Discover() []PluginEntry {
	entries, err := os.ReadDir(r.pluginsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("scan plugins dir: %v", err)
		}
		return nil
	}
	var found []PluginEntry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(r.pluginsDir, e.Name())
		m, err := LoadManifest(dir)
		entry := PluginEntry{Folder: e.Name(), Dir: dir, Manifest: m, Err: err}
		if err == nil {
			entry.Manifest = m
		}
		found = append(found, entry)
	}
	return found
}

func (r *PluginRegistry) Find(name string) (PluginEntry, bool) {
	for _, e := range r.Discover() {
		if e.Folder == name {
			return e, true
		}
	}
	return PluginEntry{}, false
}

// PluginWidgetInfo is a widget a plugin provides. Module is the API URL
// the dashboard imports the ESM module from.
type PluginWidgetInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Module string `json:"module"`
}

// PluginInfo is one valid plugin in the listing.
type PluginInfo struct {
	Name    string             `json:"name"`
	Version string             `json:"version"`
	Widgets []PluginWidgetInfo `json:"widgets"`
	Backend bool               `json:"backend"`
}

// PluginError reports an invalid dropped folder without breaking the list.
type PluginError struct {
	Folder string `json:"folder"`
	Error  string `json:"error"`
}

type PluginList struct {
	Plugins []PluginInfo  `json:"plugins"`
	Errors  []PluginError `json:"errors"`
}

func moduleURL(pluginName, modulePath string) string {
	return "/api/v1/plugins/" + url.PathEscape(pluginName) + "/modules/" + modulePath
}

func listPluginsHandler(registry *PluginRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := PluginList{Plugins: []PluginInfo{}, Errors: []PluginError{}}
		for _, entry := range registry.Discover() {
			if entry.Err != nil {
				list.Errors = append(list.Errors, PluginError{Folder: entry.Folder, Error: entry.Err.Error()})
				continue
			}
			info := PluginInfo{
				Name:    entry.Manifest.Name,
				Version: entry.Manifest.Version,
				Widgets: []PluginWidgetInfo{},
				Backend: entry.Manifest.Backend != nil,
			}
			for _, mw := range entry.Manifest.Widgets {
				info.Widgets = append(info.Widgets, PluginWidgetInfo{
					ID:     mw.ID,
					Title:  mw.Title,
					Module: moduleURL(entry.Manifest.Name, mw.Module),
				})
			}
			list.Plugins = append(list.Plugins, info)
		}
		writeJSON(w, http.StatusOK, list)
	}
}

func (r *PluginRegistry) serveModule(w http.ResponseWriter, req *http.Request, name, modulePath string) {
	entry, ok := r.Find(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found: " + name})
		return
	}
	if entry.Err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": entry.Err.Error()})
		return
	}
	file, err := entry.Manifest.ModuleFile(entry.Dir, modulePath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	f, err := os.Open(file)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "module file not found"})
		return
	}
	defer f.Close()
	switch strings.ToLower(filepath.Ext(file)) {
	case ".js", ".mjs":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	info, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stat module file"})
		return
	}
	http.ServeContent(w, req, info.Name(), info.ModTime(), f)
}

func pluginModuleHandler(registry *PluginRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		registry.serveModule(w, req, req.PathValue("name"), req.PathValue("rest"))
	}
}
