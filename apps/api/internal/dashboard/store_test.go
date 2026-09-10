package dashboard

import (
	"os"
	"path/filepath"
	"testing"
)

// These drive the Store directly rather than through the HTTP surface: they
// are about what a document on disk becomes when it is read, which is the
// store's interface.

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

// Documents written by the first persist build carried one flat theme. The
// v2 shape cannot read those colours, so both palettes come back as the
// DESIGN.md defaults and the stored mode survives.
func TestStoreLoadsLegacyFlatThemeAsDefaults(t *testing.T) {
	dataDir := t.TempDir()
	legacy := `{
	  "widgets": [{"id":"w","plugin":"sample","size":"small","enabled":true,"pollSeconds":10,"config":{"label":"Sample A"}}],
	  "theme": {"mode":"light","background":"#fffbf0","foreground":"#333","accent":"#c33"}
	}`
	if err := os.WriteFile(filepath.Join(dataDir, "dashboard.json"), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy doc: %v", err)
	}
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Theme.Light != DefaultLightPalette() {
		t.Errorf("light palette = %+v, want the default %+v", got.Theme.Light, DefaultLightPalette())
	}
	if got.Theme.Dark != DefaultDarkPalette() {
		t.Errorf("dark palette = %+v, want default %+v", got.Theme.Dark, DefaultDarkPalette())
	}
	if got.Theme.Mode != "light" {
		t.Errorf("mode = %q, want the stored light", got.Theme.Mode)
	}
	if len(got.Widgets) != 1 {
		t.Fatalf("widgets = %+v, want one", got.Widgets)
	}
	w := got.Widgets[0]
	if w.Plugin != "hello-widget" || w.Widget != "hello" {
		t.Errorf("sample widget not migrated: %+v", w)
	}
	if string(w.Config) != `{"message":"Sample A"}` {
		t.Errorf("config not migrated to message: %s", w.Config)
	}
}

// Documents saved with pre-design 3-colour palettes also land on the
// defaults, because none of those keys exist in the v2 Palette.
func TestStoreLoadsOldThreeColorPalettesAsDefaults(t *testing.T) {
	dataDir := t.TempDir()
	doc := `{
	  "widgets": [{"id":"w","plugin":"echo","size":"small","enabled":true,"pollSeconds":10}],
	  "theme": {"mode":"dark",
	    "light": {"background":"#fffbf0","foreground":"#333","accent":"#c33"},
	    "dark": {"background":"#000","foreground":"#eee","accent":"#0af"}}
	}`
	if err := os.WriteFile(filepath.Join(dataDir, "dashboard.json"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write old doc: %v", err)
	}
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Theme.Light != DefaultLightPalette() {
		t.Errorf("light palette = %+v, want the default %+v", got.Theme.Light, DefaultLightPalette())
	}
	if got.Theme.Dark != DefaultDarkPalette() {
		t.Errorf("dark palette = %+v, want the default %+v", got.Theme.Dark, DefaultDarkPalette())
	}
	if got.Theme.Mode != "dark" {
		t.Errorf("mode = %q, want the stored dark", got.Theme.Mode)
	}
}

// The palette shipped in v1 carried 29 tokens. A document saved with those
// keys must load as the fourteen DESIGN.md defaults, in both modes.
func TestStoreLoadsV1PaletteAsDefaults(t *testing.T) {
	const v1Palette = `{"primary":"#f7a501","primaryPressed":"#dd9001",` +
		`"primaryActive":"#b17816","onPrimary":"#23251d","ink":"#23251d",` +
		`"body":"#4d4f46","charcoal":"#33342d","mute":"#6c6e63","ash":"#9b9c92",` +
		`"stone":"#b6b7af","hairline":"#bfc1b7","hairlineSoft":"#dcdfd2",` +
		`"onDark":"#ffffff","canvas":"#eeefe9","surfaceSoft":"#e5e7e0",` +
		`"surfaceCard":"#ffffff","surfaceDoc":"#fcfcfa","surfaceDark":"#23251d",` +
		`"linkBlue":"#1d4ed8","linkTeal":"#1078a3","accentBlue":"#2c84e0",` +
		`"accentBlueSoft":"#dceaf6","accentRed":"#cd4239",` +
		`"accentRedSoft":"#f7d6d3","accentGreen":"#2c8c66",` +
		`"accentGreenSoft":"#d9eddf","accentPurple":"#7c44a6",` +
		`"accentPurpleSoft":"#e7d8ee","focusRing":"rgba(59,130,246,0.5)"}`
	doc := `{"widgets": [{"id":"w","plugin":"echo","size":"small","enabled":true,"pollSeconds":10}],` +
		`"theme": {"mode":"dark","light":` + v1Palette + `,"dark":` + v1Palette + `}}`

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "dashboard.json"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write v1 doc: %v", err)
	}
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Theme.Light != DefaultLightPalette() {
		t.Errorf("light palette = %+v, want the default %+v", got.Theme.Light, DefaultLightPalette())
	}
	if got.Theme.Dark != DefaultDarkPalette() {
		t.Errorf("dark palette = %+v, want the default %+v", got.Theme.Dark, DefaultDarkPalette())
	}
}
