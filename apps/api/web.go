package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Go cannot embed anything above the package directory, so the Dashboard
// build is written straight into this folder by apps/web's Vite config. The
// placeholder keeps the pattern matching on a checkout that never built one,
// which is why the handler below has something to say about a missing build.
//
//go:embed all:webdist
var webFiles embed.FS

// dashboardFS roots the embedded build at the folder holding index.html.
func dashboardFS() (fs.FS, error) {
	return fs.Sub(webFiles, "webdist")
}

// webHandler serves the embedded Dashboard on the API's own port. Anything
// under /api/ reaching this handler matched no route and stays JSON, and a
// path that looks like an asset never answers with the shell — the browser
// would try to parse HTML as a module and report something unrelated.
func webHandler(root fs.FS) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such route: " + r.URL.Path})
			return
		}
		rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if rel == "" || rel == "." {
			serveShell(w, r, root)
			return
		}
		if _, err := fs.Stat(root, rel); err != nil {
			if path.Ext(rel) == "" {
				serveShell(w, r, root)
				return
			}
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// serveShell answers with index.html, or says plainly that this binary was
// built without a Dashboard.
func serveShell(w http.ResponseWriter, r *http.Request, root fs.FS) {
	if _, err := fs.Stat(root, "index.html"); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(
			"Star Panel: this binary carries no Dashboard build. Run `pnpm build` at the repo root and rebuild.\n",
		))
		return
	}
	http.ServeFileFS(w, r, root, "index.html")
}
