package dashboard

import (
	"os"
	"path/filepath"
	"testing"
)

// These drive the Store directly rather than through the HTTP surface: they
// are about what a document on disk becomes when it is read, which is the
// store's interface.

func storeHolding(t *testing.T, document string) *Store {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dashboard.json"), []byte(document), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

// A fresh install seeds this document, so the store has to accept it.
func TestDefaultDashboardIsStorable(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if _, err := store.Save(DefaultDashboard()); err != nil {
		t.Fatalf("the seeded Dashboard was refused: %v", err)
	}
}

// Save answers with the document as stored, so a caller can trust what it
// gets back without reading the file again.
func TestSaveAnswersWithTheStoredDocument(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	saved, err := store.Save(Dashboard{
		Widgets: []Widget{{ID: "solo", Plugin: "echo", Size: "small", Enabled: true}},
		Theme:   "dracula",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Widgets[0].PollSeconds != DefaultPollSeconds {
		t.Errorf("pollSeconds = %d, want the default", saved.Widgets[0].PollSeconds)
	}
	if saved.Theme != "dracula" {
		t.Errorf("theme = %q, want the name that was saved", saved.Theme)
	}
}

// The shape the panel shipped before Themes were stylesheets: a mode and two
// palettes. The colours are gone, but the widgets the owner arranged are not,
// and the Dashboard lands on its default Theme instead of failing to load.
func TestStoreLoadsAPaletteDocumentAsTheDefaultTheme(t *testing.T) {
	store := storeHolding(t, `{
	  "widgets": [{"id":"w","plugin":"echo","size":"small","enabled":true,"pollSeconds":10}],
	  "theme": {"mode":"dark","light":{"canvas":"#f0f"},"dark":{"canvas":"#000"}}
	}`)

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Theme != DefaultTheme {
		t.Errorf("theme = %q, want %q", got.Theme, DefaultTheme)
	}
	if len(got.Widgets) != 1 {
		t.Errorf("widgets = %+v, want the one that was saved", got.Widgets)
	}
}

// The oldest shape stored a single flat three-colour theme instead.
func TestStoreLoadsAFlatThemeDocumentAsTheDefaultTheme(t *testing.T) {
	store := storeHolding(t, `{
	  "widgets": [{"id":"w","plugin":"sample","size":"small","enabled":true,"pollSeconds":10,"config":{"label":"Sample A"}}],
	  "theme": {"mode":"light","background":"#fffbf0","foreground":"#333","accent":"#c33"}
	}`)

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Theme != DefaultTheme {
		t.Errorf("theme = %q, want %q", got.Theme, DefaultTheme)
	}
	widget := got.Widgets[0]
	if widget.Plugin != "hello-widget" || widget.Widget != "hello" {
		t.Errorf("sample widget not migrated: %+v", widget)
	}
	if string(widget.Config) != `{"message":"Sample A"}` {
		t.Errorf("config not migrated to message: %s", widget.Config)
	}
}

// Whatever cannot be rendered is refused at the seam rather than half-served.
func TestStoreRefusesAnInvalidDocument(t *testing.T) {
	store := storeHolding(t, `{
	  "widgets": [{"id":"a","plugin":"echo","size":"huge","enabled":true,"pollSeconds":10}],
	  "theme": "default"
	}`)
	if _, err := store.Load(); err == nil {
		t.Fatal("a document with a bad widget size was served")
	}
}
