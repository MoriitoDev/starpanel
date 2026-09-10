package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const restartBackoff = 2 * time.Second
const startWaitTimeout = 3 * time.Second

// Supervisor starts plugin backend subprocesses, restarts them while core
// runs, and proxies the per-plugin /proxy/ path to their localhost HTTP.
// Builtins are core-provided handlers answering for plugins without a
// subprocess (the built-in system-stats plugin).
type Supervisor struct {
	registry *PluginRegistry
	builtins map[string]http.Handler

	mu    sync.Mutex
	procs map[string]*backendProc
	base  context.Context
}

func NewSupervisor(registry *PluginRegistry, builtins map[string]http.Handler) *Supervisor {
	return &Supervisor{
		registry: registry,
		builtins: builtins,
		procs:    map[string]*backendProc{},
		base:     context.Background(),
	}
}

// StartDeclared launches subprocesses for every valid plugin that declares
// a backend. Failures are logged and never stop core.
func (s *Supervisor) StartDeclared() {
	for _, entry := range s.registry.Discover() {
		if entry.Err != nil || entry.Manifest.Backend == nil {
			continue
		}
		if err := s.Ensure(entry.Manifest.Name); err != nil {
			log.Printf("plugin %q backend not started: %v", entry.Manifest.Name, err)
		}
	}
}

// StopAll kills every subprocess; called on core shutdown.
func (s *Supervisor) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, p := range s.procs {
		p.cancel()
		delete(s.procs, name)
	}
}

type backendProc struct {
	port   int
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *backendProc) running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

var errNoBackend = errors.New("plugin has no backend")

// Ensure starts the subprocess for a plugin if it is not running yet.
func (s *Supervisor) Ensure(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.procs[name]; ok && p.running() {
		return nil
	}
	entry, ok := s.registry.Find(name)
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	if entry.Err != nil {
		return entry.Err
	}
	if entry.Manifest.Backend == nil {
		return fmt.Errorf("%w: %s", errNoBackend, name)
	}
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("allocate port: %w", err)
	}
	ctx, cancel := context.WithCancel(s.base)
	p := &backendProc{port: port, cancel: cancel, done: make(chan struct{})}
	s.procs[name] = p
	go runSupervised(ctx, p.done, name, entry.Dir, entry.Manifest.Backend.Command, port)
	if err := waitUntilListening(port, startWaitTimeout); err != nil {
		// The restart loop keeps trying; the proxy reports the outage.
		return fmt.Errorf("backend %s did not accept connections: %w", name, err)
	}
	return nil
}

func (s *Supervisor) port(name string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.procs[name]
	if !ok {
		return 0, false
	}
	return p.port, true
}

// runSupervised runs the backend command, replacing {port} in arguments,
// and restarts it until core shuts down.
func runSupervised(ctx context.Context, done chan<- struct{}, name, dir string, command []string, port int) {
	defer close(done)
	for {
		args := make([]string, len(command)-1)
		for i, a := range command[1:] {
			args[i] = strings.ReplaceAll(a, "{port}", strconv.Itoa(port))
		}
		cmd := exec.CommandContext(ctx, command[0], args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"STAR_PANEL_PORT="+strconv.Itoa(port),
			"STAR_PANEL_PLUGIN="+name,
		)
		output, err := cmd.CombinedOutput()
		if len(output) > 0 {
			log.Printf("plugin %q backend output: %s", name, strings.TrimSpace(string(output)))
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err != nil {
			log.Printf("plugin %q backend exited (%v); restarting in %s", name, err, restartBackoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(restartBackoff):
		}
	}
}

func (s *Supervisor) proxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		rest := r.PathValue("rest")
		entry, ok := s.registry.Find(name)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found: " + name})
			return
		}
		if entry.Err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "plugin manifest invalid: " + entry.Err.Error()})
			return
		}
		// Core-provided plugins answer in process, no subprocess needed.
		if builtin, ok := s.builtins[name]; ok {
			forwarded := r.Clone(r.Context())
			forwarded.URL.Path = "/" + rest
			forwarded.URL.RawPath = ""
			builtin.ServeHTTP(w, forwarded)
			return
		}
		if entry.Manifest.Backend == nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("plugin %q has no backend", name)})
			return
		}
		if err := s.Ensure(name); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "plugin backend unavailable: " + err.Error()})
			return
		}
		port, ok := s.port(name)
		if !ok {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "plugin backend unavailable"})
			return
		}
		target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(port)}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "plugin backend unavailable: " + err.Error()})
		}
		// The backend sees its own paths: strip the proxy prefix.
		forwarded := r.Clone(r.Context())
		forwarded.URL.Path = "/" + rest
		proxy.ServeHTTP(w, forwarded)
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitUntilListening(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := "127.0.0.1:" + strconv.Itoa(port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}
