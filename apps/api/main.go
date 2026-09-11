package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"star-panel/internal/dashboard"
	"star-panel/internal/plugins"
	"star-panel/internal/themes"
)

// resolveDir locates the data or plugins folder. Left alone both live beside
// the binary, which is what a deployed Star Panel relies on; naming one
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

// main is the composition root: it turns flags into folders, wires the two
// domain modules and core's built-in Plugin into the HTTP surface, and gets
// out of the way.
func main() {
	dataDir := flag.String("data-dir", "", "directory holding the dashboard document (default: beside the binary)")
	pluginsDir := flag.String("plugins-dir", "", "directory of dropped plugin folders (default: beside the binary)")
	themesDir := flag.String("themes-dir", "", "directory of imported theme stylesheets (default: beside the binary)")
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
	resolvedThemes, err := resolveDir(*themesDir, "themes")
	if err != nil {
		log.Fatalf("resolve themes dir: %v", err)
	}

	store, err := dashboard.NewStore(resolvedData)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	registry := plugins.NewRegistry(resolvedPlugins)
	themeStore, err := themes.NewStore(resolvedThemes)
	if err != nil {
		log.Fatalf("init themes: %v", err)
	}
	// Built-ins are core's own Plugins: they answer in process, and only the
	// composition root knows they exist.
	backends := plugins.NewBackends(registry, map[string]http.Handler{
		"system-stats": newStatsHandler(newStatsSource()),
	})
	backends.Start()

	web, err := dashboardFS()
	if err != nil {
		log.Fatalf("open embedded dashboard: %v", err)
	}

	server := &http.Server{
		Addr: *addr,
		Handler: newServer(serverDeps{
			store:    store,
			registry: registry,
			backends: backends,
			themes:   themeStore,
			web:      web,
		}),
	}
	go func() {
		<-ctx.Done()
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		backends.Stop()
	}()

	log.Printf("star panel on %s (data dir: %s, plugins dir: %s, themes dir: %s)",
		*addr, resolvedData, resolvedPlugins, resolvedThemes)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
