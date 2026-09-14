package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"star-panel/internal/dashboard"
	"star-panel/internal/plugins"
	"star-panel/internal/themes"
)

// serverDeps is everything the HTTP surface needs. A struct rather than
// positional arguments, so a new dependency does not churn every caller, and
// every test assembles the server the way core does.
type serverDeps struct {
	store    *dashboard.Store
	registry *plugins.Registry
	backends *plugins.Backends
	themes   *themes.Store
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
	mux.HandleFunc("POST /api/v1/plugins", importPluginHandler(deps.registry))
	mux.HandleFunc("GET /api/v1/plugins/{name}/archive", pluginArchiveHandler(deps.registry))
	mux.HandleFunc("GET /api/v1/plugins/{name}/modules/{rest...}", pluginModuleHandler(deps.registry))
	mux.HandleFunc("/api/v1/plugins/{name}/proxy/{rest...}", proxyHandler(deps.backends))
	mux.HandleFunc("GET /api/v1/themes", listThemesHandler(deps))
	mux.HandleFunc("POST /api/v1/themes", importThemeHandler(deps.themes))
	mux.HandleFunc("DELETE /api/v1/themes/{slug}", deleteThemeHandler(deps.themes))
	mux.HandleFunc("GET /api/v1/themes/{slug}", serveThemeHandler(deps.themes))
	mux.Handle("/", webHandler(deps.web, activeThemeHref(deps)))
	return mux
}

// maxThemeBytes caps an import. There is nothing to validate inside a
// stylesheet — whatever the author wants is the author's business — but a
// panel that reads a request body should know how much it will hold.
const maxThemeBytes = 1 << 20

// ThemeInfo is one row of the theme listing: what to call it, what to ask for
// it by, and whether its file is really there.
type ThemeInfo struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Present bool   `json:"present"`
}

type ThemeList struct {
	Active string      `json:"active"`
	Themes []ThemeInfo `json:"themes"`
}

// activeThemeName reads the Dashboard's choice. A document that cannot be read
// falls back to the baseline: the theme listing is not the place to report a
// broken document, and the panel still has to paint something.
func activeThemeName(deps serverDeps) string {
	document, err := deps.store.Load()
	if err != nil || document.Theme == "" {
		return dashboard.DefaultTheme
	}
	return string(document.Theme)
}

// activeThemeHref is the stylesheet the shell should link, or "" when the
// Dashboard renders with the baseline. A name with no file behind it is not an
// error to shout about here — the theme list is where that shows — but it must
// never produce a link to nothing.
func activeThemeHref(deps serverDeps) func() string {
	return func() string {
		slug := activeThemeName(deps)
		if slug == dashboard.DefaultTheme {
			return ""
		}
		if _, ok := deps.themes.Find(slug); !ok {
			return ""
		}
		return "/api/v1/themes/" + url.PathEscape(slug) + ".css"
	}
}

func listThemesHandler(deps serverDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		active := activeThemeName(deps)
		list := ThemeList{
			Active: active,
			Themes: []ThemeInfo{{Name: "Default", Slug: dashboard.DefaultTheme, Present: true}},
		}
		for _, theme := range deps.themes.List() {
			list.Themes = append(list.Themes, ThemeInfo{
				Name:    theme.Name,
				Slug:    theme.Slug,
				Present: true,
			})
		}
		present := false
		for _, theme := range list.Themes {
			if theme.Slug == active {
				present = true
			}
		}
		if !present {
			// The owner needs to know their Theme is gone rather than find it
			// quietly dropped from the list.
			list.Themes = append(list.Themes, ThemeInfo{Name: active, Slug: active, Present: false})
		}
		writeJSON(w, http.StatusOK, list)
	}
}

func importThemeHandler(store *themes.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		css, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxThemeBytes))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the theme is too large to import"})
			return
		}
		imported, err := store.Add(css)
		switch {
		case errors.Is(err, themes.ErrExists):
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "a theme called " + imported.Name + " is already imported; rename this one or delete that one first",
			})
		case err != nil:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusCreated, imported)
		}
	}
}

func deleteThemeHandler(store *themes.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		if slug == dashboard.DefaultTheme {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "the default theme is not a file, so there is nothing to delete",
			})
			return
		}
		switch err := store.Delete(slug); {
		case errors.Is(err, themes.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "theme not found: " + slug})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// serveThemeHandler answers the URL the shell links and the download button
// points at. The slug arrives straight from a URL, so the store only resolves
// it against files it found itself.
func serveThemeHandler(store *themes.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimSuffix(r.PathValue("slug"), ".css")
		css, ok := store.Read(slug)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "theme not found: " + slug})
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(css)
	}
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
	// Problem says why this Plugin cannot run here, in the words a person
	// reads. Empty when nothing is wrong.
	Problem string `json:"problem,omitempty"`
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
			list.Plugins = append(list.Plugins, pluginInfo(entry))
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// pluginInfo is one Plugin as the panel reads it. The listing and an import
// both answer with this, so a Plugin says the same thing about itself however
// it arrived.
func pluginInfo(entry plugins.Entry) PluginInfo {
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
	if missing := entry.Missing(); len(missing) > 0 {
		info.Problem = requirementProblem(missing)
	}
	return info
}

// maxPluginArchiveBytes caps an import. A Plugin folder may carry a compiled
// binary — that is the author's business, and docs/PLUGINS.md says so — which
// is why this is larger than the stylesheet a Theme arrives as, and still a
// number this process is willing to hold.
const maxPluginArchiveBytes = 32 << 20

// importPluginHandler takes a Plugin as a ZIP. A Theme is one file and arrives
// as a request body; a Plugin is a folder, so it arrives as an archive — but
// the shape of the exchange is the same one, and the name comes from inside
// the artefact either way.
func importPluginHandler(registry *plugins.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		archive, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPluginArchiveBytes))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "the plugin archive is too large to import",
			})
			return
		}
		// Unpacking, validating and moving into place all happen behind the
		// Plugins module; the transport only decides which failure is which.
		entry, err := registry.Import(archive)
		switch {
		case errors.Is(err, plugins.ErrExists):
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		case err != nil:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusCreated, pluginInfo(entry))
		}
	}
}

// pluginArchiveHandler answers with a Plugin's folder as a ZIP, which the
// panel's download button saves and an import takes back.
func pluginArchiveHandler(registry *plugins.Registry) http.HandlerFunc {
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
		archive, err := entry.Archive()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+entry.Folder+`.zip"`)
		_, _ = w.Write(archive)
	}
}

// requirementProblem turns what a Plugin is missing into a sentence. The panel
// never installs anything — no auth, no privileges, and an unauthenticated
// endpoint that runs package managers is a hole — so it says what is missing
// and docs/PLUGINS.md says what to do about it.
func requirementProblem(missing []plugins.Requirement) string {
	labels := make([]string, 0, len(missing))
	for _, required := range missing {
		label := required.Command
		if required.MinVersion != "" {
			label += " >= " + required.MinVersion
		}
		labels = append(labels, label)
	}
	return "needs " + strings.Join(labels, ", ") + ", which is not on this machine"
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
		// Files behind this URL are replaceable — an owner re-imports a Plugin
		// over the folder it was unzipped from — so the browser is told to ask
		// again rather than keep the copy it has. ServeContent answers the
		// question with a 304 while the file is untouched, so the cost of
		// asking is what it should be.
		w.Header().Set("Cache-Control", "no-cache")
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
