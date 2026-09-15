package dashboard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		Widgets: []Widget{{ID: "solo", Plugin: "echo", Span: Span{W: 4, H: 2}, Enabled: true}},
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
	  "widgets": [{"id":"a","plugin":"echo","w":13,"h":1,"enabled":true,"pollSeconds":10}],
	  "theme": "default"
	}`)
	if _, err := store.Load(); err == nil {
		t.Fatal("a document with a Widget wider than the grid was served")
	}
}

// A Widget used to declare one of three sizes. A document written then still
// has to come back, as the Span that replaced them (ADR-0008).
func TestStoreMigratesTheOldSizesToSpans(t *testing.T) {
	store := storeHolding(t, `{
	  "widgets": [
	    {"id":"a","plugin":"echo","size":"small","enabled":true,"pollSeconds":10},
	    {"id":"b","plugin":"echo","size":"medium","enabled":true,"pollSeconds":10},
	    {"id":"c","plugin":"echo","size":"large","enabled":true,"pollSeconds":10}
	  ],
	  "theme": "default"
	}`)

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := map[string][2]int{"a": {4, 1}, "b": {6, 1}, "c": {12, 1}}
	for _, widget := range got.Widgets {
		if want[widget.ID] != [2]int{widget.W, widget.H} {
			t.Errorf("widget %q = %d columns by %d rows, want %v",
				widget.ID, widget.W, widget.H, want[widget.ID])
		}
	}
}

// The retired field is read, never written: the document on disk is a Span
// document the moment it has been through the store.
func TestStoreDoesNotWriteTheRetiredSizeBackOut(t *testing.T) {
	dir := t.TempDir()
	document := `{"widgets":[{"id":"a","plugin":"echo","size":"large","enabled":true,"pollSeconds":10}],"theme":"default"}`
	if err := os.WriteFile(filepath.Join(dir, "dashboard.json"), []byte(document), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, err := store.Save(loaded); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "dashboard.json"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(raw), `"size"`) {
		t.Errorf("the stored document still carries a size: %s", raw)
	}
}

// A tab opened before Spans PUTs the old shape. The store refuses it and names
// what to send instead, rather than guessing at a size it can still read
// (ADR-0008: refused, not rescued).
func TestStoreRefusesALegacyDocumentOnSave(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	_, err = store.Save(Dashboard{
		Widgets: []Widget{{ID: "a", Plugin: "echo", Size: "medium", Enabled: true, PollSeconds: 10}},
		Theme:   DefaultTheme,
	})
	if err == nil {
		t.Fatal("a document carrying the retired size was saved")
	}
	var invalid *InvalidError
	if !errors.As(err, &invalid) {
		t.Errorf("error = %v, want an InvalidError a caller can tell from a storage failure", err)
	}
	if !strings.Contains(err.Error(), "w and h") {
		t.Errorf("error = %q, want the fields to send named", err)
	}
}

// Every Widget fills at least one cell and no more than the twelve the widest
// grid has.
func TestStoreRefusesSpansOutsideTheGrid(t *testing.T) {
	cases := map[string]Widget{
		"no columns": {ID: "a", Plugin: "echo", Span: Span{W: 0, H: 1}, Enabled: true, PollSeconds: 10},
		"too wide":   {ID: "a", Plugin: "echo", Span: Span{W: 13, H: 1}, Enabled: true, PollSeconds: 10},
		"no rows":    {ID: "a", Plugin: "echo", Span: Span{W: 6, H: 0}, Enabled: true, PollSeconds: 10},
		"too tall":   {ID: "a", Plugin: "echo", Span: Span{W: 6, H: 13}, Enabled: true, PollSeconds: 10},
	}
	for name, widget := range cases {
		t.Run(name, func(t *testing.T) {
			store, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatalf("new store: %v", err)
			}
			if _, err := store.Save(Dashboard{Widgets: []Widget{widget}, Theme: DefaultTheme}); err == nil {
				t.Fatalf("a Widget of %d columns by %d rows was stored", widget.W, widget.H)
			}
		})
	}
}
