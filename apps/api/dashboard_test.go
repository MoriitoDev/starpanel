package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	registry := NewPluginRegistry(t.TempDir())
	supervisor := NewSupervisor(registry, nil)
	t.Cleanup(supervisor.StopAll)
	return httptest.NewServer(newMux(store, registry, supervisor)), dataDir
}

func doJSON(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	var decoded map[string]any
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return res, decoded
}

func TestDashboardGetReturnsDefaultShape(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/dashboard", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	theme, ok := body["theme"].(map[string]any)
	if !ok {
		t.Fatalf("theme missing or wrong type: %v", body["theme"])
	}
	if theme["mode"] != "auto" {
		t.Errorf("theme.mode = %v, want auto", theme["mode"])
	}
	tokenKeys := []string{"canvas", "surface", "surfaceSoft", "border",
		"borderSoft", "ink", "body", "mute", "accent", "accentPress",
		"onAccent", "danger", "success", "focusRing"}
	for _, name := range []string{"light", "dark"} {
		palette, ok := theme[name].(map[string]any)
		if !ok {
			t.Fatalf("theme.%s missing or wrong type: %v", name, theme[name])
		}
		for _, key := range tokenKeys {
			if color, _ := palette[key].(string); color == "" {
				t.Errorf("theme.%s.%s is empty", name, key)
			}
		}
	}
	widgets, ok := body["widgets"].([]any)
	if !ok || len(widgets) == 0 {
		t.Fatalf("widgets missing or empty: %v", body["widgets"])
	}
	first, _ := widgets[0].(map[string]any)
	for _, key := range []string{"id", "plugin", "size", "enabled", "pollSeconds"} {
		if _, present := first[key]; !present {
			t.Errorf("widget missing key %q", key)
		}
	}
}

func TestDashboardSaveGetRoundTrip(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	saved := Dashboard{
		Widgets: []Widget{
			{ID: "clock", Plugin: "sysmon", Size: "large", Enabled: true, Config: json.RawMessage(`{"tz":"UTC"}`)},
			{ID: "notes", Plugin: "sysmon", Size: "small", Enabled: false},
		},
		Theme: Theme{
			Mode:  "auto",
			Light: defaultLightPalette(),
			Dark:  defaultDarkPalette(),
		},
	}
	res, _ := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", saved)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d, want 200", res.StatusCode)
	}

	// Save must default the unset poll interval.
	saved.Widgets[0].PollSeconds = defaultPollSeconds
	saved.Widgets[1].PollSeconds = defaultPollSeconds

	res, body := doJSON(t, ts, http.MethodGet, "/api/v1/dashboard", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", res.StatusCode)
	}
	raw, _ := json.Marshal(body)
	var got Dashboard
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("redecode dashboard: %v", err)
	}
	if fmt.Sprint(got) != fmt.Sprint(saved) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, saved)
	}
	if len(got.Widgets) != 2 || got.Widgets[0].ID != "clock" {
		t.Errorf("widget order not preserved: %+v", got.Widgets)
	}
}

func TestDashboardSaveRejectsInvalidDocuments(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	fullLight := defaultLightPalette()
	fullDark := defaultDarkPalette()
	noAccent := defaultDarkPalette()
	noAccent.Accent = ""

	cases := map[string]Dashboard{
		"empty widget id": {
			Widgets: []Widget{{ID: "", Plugin: "sysmon", Size: "small", Enabled: true}},
			Theme:   Theme{Mode: "dark", Light: fullLight, Dark: fullDark},
		},
		"duplicate widget ids": {
			Widgets: []Widget{
				{ID: "a", Plugin: "sysmon", Size: "small", Enabled: true},
				{ID: "a", Plugin: "sysmon", Size: "small", Enabled: true},
			},
			Theme: Theme{Mode: "dark", Light: fullLight, Dark: fullDark},
		},
		"bad size": {
			Widgets: []Widget{{ID: "a", Plugin: "sysmon", Size: "huge", Enabled: true}},
			Theme:   Theme{Mode: "dark", Light: fullLight, Dark: fullDark},
		},
		"bad theme mode": {
			Widgets: []Widget{{ID: "a", Plugin: "sysmon", Size: "small", Enabled: true}},
			Theme:   Theme{Mode: "solarized", Light: fullLight, Dark: fullDark},
		},
		"missing dark accent": {
			Widgets: []Widget{{ID: "a", Plugin: "sysmon", Size: "small", Enabled: true}},
			Theme: Theme{
				Mode:  "dark",
				Light: fullLight,
				Dark:  noAccent,
			},
		},
		"missing light palette": {
			Widgets: []Widget{{ID: "a", Plugin: "sysmon", Size: "small", Enabled: true}},
			Theme:   Theme{Mode: "light", Dark: fullDark},
		},
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			res, body := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", doc)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", res.StatusCode)
			}
			if _, ok := body["error"]; !ok {
				t.Errorf("expected error field in body, got %v", body)
			}
		})
	}
}

func TestDashboardSaveRejectsMalformedJSON(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/dashboard", bytes.NewBufferString("{not json"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

func TestDashboardPersistsAcrossRestart(t *testing.T) {
	ts, dataDir := newTestServer(t)

	saved := Dashboard{
		Widgets: []Widget{{ID: "solo", Plugin: "sysmon", Size: "medium", Enabled: false}},
		Theme: Theme{
			Mode:  "light",
			Light: func() Palette { p := defaultLightPalette(); p.Canvas = "#fafafa"; return p }(),
			Dark:  fullDefaultDark(),
		},
	}
	res, _ := doJSON(t, ts, http.MethodPut, "/api/v1/dashboard", saved)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d, want 200", res.StatusCode)
	}
	ts.Close()

	restarted, err := NewStore(dataDir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	got, err := restarted.Load()
	if err != nil {
		t.Fatalf("load after restart: %v", err)
	}
	saved.Widgets[0].PollSeconds = defaultPollSeconds
	if fmt.Sprint(got) != fmt.Sprint(saved) {
		t.Errorf("dashboard lost across restart:\n got %+v\nwant %+v", got, saved)
	}
}

func fullDefaultDark() Palette {
	return defaultDarkPalette()
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
	if got.Theme.Light != defaultLightPalette() {
		t.Errorf("light palette = %+v, want the default %+v", got.Theme.Light, defaultLightPalette())
	}
	if got.Theme.Dark != defaultDarkPalette() {
		t.Errorf("dark palette = %+v, want default %+v", got.Theme.Dark, defaultDarkPalette())
	}
	if got.Theme.Mode != "light" {
		t.Errorf("mode = %q, want the stored light", got.Theme.Mode)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("migrated doc does not validate: %v", err)
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
	if got.Theme.Light != defaultLightPalette() {
		t.Errorf("light palette = %+v, want the default %+v", got.Theme.Light, defaultLightPalette())
	}
	if got.Theme.Dark != defaultDarkPalette() {
		t.Errorf("dark palette = %+v, want the default %+v", got.Theme.Dark, defaultDarkPalette())
	}
	if got.Theme.Mode != "dark" {
		t.Errorf("mode = %q, want the stored dark", got.Theme.Mode)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("migrated doc does not validate: %v", err)
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
	if got.Theme.Light != defaultLightPalette() {
		t.Errorf("light palette = %+v, want the default %+v", got.Theme.Light, defaultLightPalette())
	}
	if got.Theme.Dark != defaultDarkPalette() {
		t.Errorf("dark palette = %+v, want the default %+v", got.Theme.Dark, defaultDarkPalette())
	}
	if err := got.Validate(); err != nil {
		t.Errorf("migrated doc does not validate: %v", err)
	}
}
