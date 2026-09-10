package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"star-panel/internal/dashboard"
	"star-panel/internal/plugins"
)

// serverDeps is everything the HTTP surface needs. A struct rather than
// positional arguments, so a new dependency does not churn every caller, and
// every test assembles the server the way core does.
type serverDeps struct {
	store    *dashboard.Store
	registry *plugins.Registry
	backends *plugins.Backends
	web      fs.FS
}

// newServer is the whole HTTP surface: the v1 routes, the embedded Dashboard,
// and the JSON shapes and status codes they answer with. One fact — what v1 is
// — lives in one place, and tests cross the same seam production does.
func newServer(deps serverDeps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", healthHandler)
	mux.HandleFunc("GET /api/v1/dashboard", getDashboardHandler(deps.store))
	mux.HandleFunc("PUT /api/v1/dashboard", saveDashboardHandler(deps.store))
	mux.HandleFunc("GET /api/v1/plugins", listPluginsHandler(deps.registry))
	mux.HandleFunc("GET /api/v1/plugins/{name}/modules/{rest...}", pluginModuleHandler(deps.registry))
	mux.HandleFunc("/api/v1/plugins/{name}/proxy/{rest...}", proxyHandler(deps.backends))
	mux.Handle("/", webHandler(deps.web))
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": "v1"})
}

func getDashboardHandler(store *dashboard.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		document, err := store.Load()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, document)
	}
}

func saveDashboardHandler(store *dashboard.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var document dashboard.Dashboard
		if err := json.NewDecoder(r.Body).Decode(&document); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		// The store owns normalising, validating and writing; the only
		// decision left here is which kind of failure came back.
		saved, err := store.Save(document)
		var invalid *dashboard.InvalidError
		switch {
		case errors.As(err, &invalid):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusOK, saved)
		}
	}
}

// proxyHandler forwards a Plugin's traffic to whatever backend the Plugins
// module hands back. Stripping the proxy prefix is the transport's job: a
// backend sees its own paths, and that is part of the Plugin contract.
func proxyHandler(backends *plugins.Backends) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler, err := backends.Handler(r.PathValue("name"))
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, plugins.ErrPluginNotFound) {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		forwarded := r.Clone(r.Context())
		forwarded.URL.Path = "/" + r.PathValue("rest")
		forwarded.URL.RawPath = ""
		handler.ServeHTTP(w, forwarded)
	}
}

// PluginWidgetInfo is a Widget a Plugin provides. Module is the URL the
// Dashboard imports the ESM module from, which is the transport's spelling of
// the folder-relative path the Manifest declares.
type PluginWidgetInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Module string `json:"module"`
}

// PluginInfo is one valid Plugin in the listing.
type PluginInfo struct {
	Name    string             `json:"name"`
	Version string             `json:"version"`
	Widgets []PluginWidgetInfo `json:"widgets"`
	Backend bool               `json:"backend"`
}

// PluginError reports a rejected folder without breaking the listing.
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

func listPluginsHandler(registry *plugins.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		list := PluginList{Plugins: []PluginInfo{}, Errors: []PluginError{}}
		for _, entry := range registry.Discover() {
			if entry.Err != nil {
				list.Errors = append(list.Errors, PluginError{
					Folder: entry.Folder,
					Error:  entry.Err.Error(),
				})
				continue
			}
			info := PluginInfo{
				Name:    entry.Manifest.Name,
				Version: entry.Manifest.Version,
				Widgets: []PluginWidgetInfo{},
				Backend: entry.Manifest.Backend != nil,
			}
			for _, widget := range entry.Manifest.Widgets {
				info.Widgets = append(info.Widgets, PluginWidgetInfo{
					ID:     widget.ID,
					Title:  widget.Title,
					Module: moduleURL(entry.Manifest.Name, widget.Module),
				})
			}
			list.Plugins = append(list.Plugins, info)
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// pluginModuleHandler serves a Widget's ESM entry out of its Plugin folder.
// Resolving the path, including the guard against escaping that folder, is
// the Plugins module's job; choosing the content type is the transport's,
// because a browser refuses to execute a module served as text/plain.
func pluginModuleHandler(registry *plugins.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		entry, ok := registry.Find(name)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found: " + name})
			return
		}
		if entry.Err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": entry.Err.Error()})
			return
		}
		file, err := entry.Manifest.ModuleFile(entry.Dir, r.PathValue("rest"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		opened, err := os.Open(file)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "module file not found"})
			return
		}
		defer opened.Close()
		if contentType := moduleContentType(file); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		info, err := opened.Stat()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stat module file"})
			return
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), opened)
	}
}

func moduleContentType(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	default:
		return ""
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
