package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// InvalidError reports a document the store refused because it does not
// validate. Callers use it to tell a caller's mistake, worth explaining to
// whoever sent it, from a storage failure, which is not.
type InvalidError struct{ Err error }

func (e *InvalidError) Error() string { return e.Err.Error() }
func (e *InvalidError) Unwrap() error { return e.Err }

// Store persists the Dashboard as a single flat JSON document — no database
// in v1 (spec: enabled set, layout, and Theme live in one document). It owns
// the invariant that a document coming back out has been migrated, normalised
// and validated, so no caller can skip any of the three.
type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &Store{path: filepath.Join(dataDir, "dashboard.json")}, nil
}

// Load reads the dashboard document, falling back to the default layout
// when nothing has been saved yet.
func (s *Store) Load() (Dashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) load() (Dashboard, error) {
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return DefaultDashboard(), nil
	}
	if err != nil {
		return Dashboard{}, fmt.Errorf("read %s: %w", s.path, err)
	}
	var d Dashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return Dashboard{}, fmt.Errorf("parse %s: %w", s.path, err)
	}
	d.migrate()
	d.normalize()
	if err := d.validate(); err != nil {
		return Dashboard{}, fmt.Errorf("%s is not a valid Dashboard: %w", s.path, err)
	}
	return d, nil
}

// Save normalises, validates and atomically writes the Dashboard, returning
// it as stored so a caller can answer with exactly what a later Load would
// give back. A document that fails validation is never written; it comes back
// as an *InvalidError.
func (s *Store) Save(d Dashboard) (Dashboard, error) {
	d.normalize()
	if err := d.validate(); err != nil {
		return Dashboard{}, &InvalidError{Err: err}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return Dashboard{}, fmt.Errorf("encode dashboard: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return Dashboard{}, fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return Dashboard{}, fmt.Errorf("replace %s: %w", s.path, err)
	}
	return d, nil
}
