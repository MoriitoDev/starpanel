package main

import (
	"bytes"
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
//
// themeHref is asked for the active Theme's stylesheet just before the shell
// goes out, so the browser starts fetching it in parallel with the app rather
// than the panel painting the baseline and flipping a moment later.
func webHandler(root fs.FS, themeHref func() string) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such route: " + r.URL.Path})
			return
		}
		rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if rel == "" || rel == "." {
			serveShell(w, root, themeHref)
			return
		}
		if _, err := fs.Stat(root, rel); err != nil {
			if path.Ext(rel) == "" {
				serveShell(w, root, themeHref)
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
func serveShell(w http.ResponseWriter, root fs.FS, themeHref func() string) {
	index, err := fs.ReadFile(root, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(
			"Star Panel: this binary carries no Dashboard build. Run `pnpm build` at the repo root and rebuild.\n",
		))
		return
	}
	if href := themeHref(); href != "" {
		// data-theme is how the app finds the link again to swap it in place.
		link := `<link rel="stylesheet" data-theme="` + strings.TrimSuffix(path.Base(href), ".css") +
			`" href="` + href + `">`
		index = bytes.Replace(index, []byte("</head>"), []byte(link+"</head>"), 1)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}
