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

// ErrPluginNotFound reports a Plugin name no folder declares, so the
// transport can answer 404 instead of blaming a backend that never existed.
var ErrPluginNotFound = errors.New("plugin not found")

// Backends resolves a Plugin to something the transport can forward /proxy/
// traffic to. A Plugin that declares a backend command runs as a supervised
// subprocess on a localhost port; a built-in answers in process. The caller
// asks for a handler and never learns which of the two it got — the port, the
// restart loop and the reverse proxy stay behind this seam.
type Backends struct {
	registry *PluginRegistry
	builtins map[string]http.Handler

	mu    sync.Mutex
	procs map[string]*backendProc
	base  context.Context
}

// NewBackends takes the built-ins from the composition root: core registers
// the Plugins it answers itself, so this module never imports one of them.
func NewBackends(registry *PluginRegistry, builtins map[string]http.Handler) *Backends {
	return &Backends{
		registry: registry,
		builtins: builtins,
		procs:    map[string]*backendProc{},
		base:     context.Background(),
	}
}

// Start launches a subprocess for every valid Plugin that declares one.
// Failures are logged and never stop core.
func (b *Backends) Start() {
	for _, entry := range b.registry.Discover() {
		if entry.Err != nil || entry.Manifest.Backend == nil {
			continue
		}
		if err := b.ensure(entry.Manifest.Name); err != nil {
			log.Printf("plugin %q backend not started: %v", entry.Manifest.Name, err)
		}
	}
}

// Stop kills every subprocess; called on core shutdown.
func (b *Backends) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for name, p := range b.procs {
		p.cancel()
		delete(b.procs, name)
	}
}

// Handler returns what to forward this Plugin's /proxy/ traffic to, or an
// error saying why it cannot answer. The handler expects a path relative to
// the backend, because the caller owns the proxy prefix.
func (b *Backends) Handler(name string) (http.Handler, error) {
	entry, ok := b.registry.Find(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}
	if entry.Err != nil {
		return nil, fmt.Errorf("plugin manifest invalid: %w", entry.Err)
	}
	// Core-provided Plugins answer in process, no subprocess needed.
	if builtin, ok := b.builtins[name]; ok {
		return builtin, nil
	}
	if entry.Manifest.Backend == nil {
		return nil, fmt.Errorf("plugin %q has no backend", name)
	}
	if err := b.ensure(name); err != nil {
		return nil, fmt.Errorf("plugin backend unavailable: %w", err)
	}
	port, ok := b.port(name)
	if !ok {
		return nil, errors.New("plugin backend unavailable")
	}
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(port)}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "plugin backend unavailable: " + err.Error()})
	}
	return proxy, nil
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

// ensure starts the subprocess for a Plugin if it is not running yet.
func (b *Backends) ensure(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if p, ok := b.procs[name]; ok && p.running() {
		return nil
	}
	entry, ok := b.registry.Find(name)
	if !ok {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}
	if entry.Err != nil {
		return entry.Err
	}
	if entry.Manifest.Backend == nil {
		return fmt.Errorf("plugin %q has no backend", name)
	}
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("allocate port: %w", err)
	}
	ctx, cancel := context.WithCancel(b.base)
	p := &backendProc{port: port, cancel: cancel, done: make(chan struct{})}
	b.procs[name] = p
	go runSupervised(ctx, p.done, name, entry.Dir, entry.Manifest.Backend.Command, port)
	if err := waitUntilListening(port, startWaitTimeout); err != nil {
		// The restart loop keeps trying; the proxy reports the outage.
		return fmt.Errorf("backend %s did not accept connections: %w", name, err)
	}
	return nil
}

func (b *Backends) port(name string) (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.procs[name]
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
