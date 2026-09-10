package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store persists the Dashboard as a single flat JSON document — no database
// in v1 (spec: enabled set, layout, and Theme live in one document).
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
	d.Migrate()
	return d, nil
}

// Save validates and atomically writes the dashboard document.
func (s *Store) Save(d Dashboard) error {
	d.Normalize()
	if err := d.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("encode dashboard: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace %s: %w", s.path, err)
	}
	return nil
}
