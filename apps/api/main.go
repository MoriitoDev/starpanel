package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": "v1"})
}

func getDashboardHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, err := store.Load()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}

func saveDashboardHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var d Dashboard
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		// The store owns normalising, validating and writing; the only
		// decision left here is which kind of failure came back.
		saved, err := store.Save(d)
		var invalid *InvalidError
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func newMux(store *Store, registry *PluginRegistry, backends *Backends) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", healthHandler)
	mux.HandleFunc("GET /api/v1/dashboard", getDashboardHandler(store))
	mux.HandleFunc("PUT /api/v1/dashboard", saveDashboardHandler(store))
	mux.HandleFunc("GET /api/v1/plugins", listPluginsHandler(registry))
	mux.HandleFunc("GET /api/v1/plugins/{name}/modules/{rest...}", pluginModuleHandler(registry))
	mux.HandleFunc("/api/v1/plugins/{name}/proxy/{rest...}", proxyHandler(backends))
	return mux
}

// proxyHandler forwards a Plugin's traffic to whatever backend the Plugins
// module hands back. Stripping the proxy prefix is the transport's job: the
// backend sees its own paths, and that is part of the Plugin contract.
func proxyHandler(backends *Backends) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler, err := backends.Handler(r.PathValue("name"))
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, ErrPluginNotFound) {
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

// resolveDir locates the data or plugins folder. Left alone both live beside
// the binary, which is what a downloaded Star Panel relies on; naming one
// explicitly keeps it relative to the working directory, which is what the
// dev commands rely on.
func resolveDir(value, name string) (string, error) {
	if value != "" {
		return filepath.Abs(value)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), name), nil
}

func main() {
	dataDir := flag.String("data-dir", "", "directory holding the dashboard document (default: beside the binary)")
	pluginsDir := flag.String("plugins-dir", "", "directory of dropped plugin folders (default: beside the binary)")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resolvedData, err := resolveDir(*dataDir, "data")
	if err != nil {
		log.Fatalf("resolve data dir: %v", err)
	}
	resolvedPlugins, err := resolveDir(*pluginsDir, "plugins")
	if err != nil {
		log.Fatalf("resolve plugins dir: %v", err)
	}

	store, err := NewStore(resolvedData)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	registry := NewPluginRegistry(resolvedPlugins)
	builtins := map[string]http.Handler{
		"system-stats": newStatsHandler(newStatsSource()),
	}
	backends := NewBackends(registry, builtins)
	backends.Start()

	web, err := dashboardFS()
	if err != nil {
		log.Fatalf("open embedded dashboard: %v", err)
	}
	mux := newMux(store, registry, backends)
	mux.Handle("/", webHandler(web))

	server := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}
	go func() {
		<-ctx.Done()
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		backends.Stop()
	}()

	log.Printf("star panel on %s (data dir: %s, plugins dir: %s)", *addr, resolvedData, resolvedPlugins)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
