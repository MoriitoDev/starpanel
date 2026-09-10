package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

const defaultPollSeconds = 10

// Widget is one entry in the Dashboard's ordered list. Config is free-form
// per-widget JSON; PollSeconds falls back to DefaultPollSeconds when unset.
// Widget optionally names which of the plugin's widgets this entry renders
// (the plugin's first widget when empty).
type Widget struct {
	ID          string          `json:"id"`
	Plugin      string          `json:"plugin"`
	Widget      string          `json:"widget,omitempty"`
	Size        string          `json:"size"`
	Enabled     bool            `json:"enabled"`
	PollSeconds int             `json:"pollSeconds"`
	Config      json.RawMessage `json:"config,omitempty"`
}

// Palette is one set of CSS variables: exactly the fourteen colour Tokens of
// DESIGN.md and nothing else. JSON names are camelCase; the CSS variables are
// their kebab-case twins (--surface-soft, --accent-press, ...).
type Palette struct {
	Canvas      string `json:"canvas"`
	Surface     string `json:"surface"`
	SurfaceSoft string `json:"surfaceSoft"`
	Border      string `json:"border"`
	BorderSoft  string `json:"borderSoft"`
	Ink         string `json:"ink"`
	Body        string `json:"body"`
	Mute        string `json:"mute"`
	Accent      string `json:"accent"`
	AccentPress string `json:"accentPress"`
	OnAccent    string `json:"onAccent"`
	Danger      string `json:"danger"`
	Success     string `json:"success"`
	FocusRing   string `json:"focusRing"`
}

// colors exposes the design tokens for validation and emptiness checks.
func (p Palette) colors() map[string]string {
	return map[string]string{
		"canvas": p.Canvas, "surface": p.Surface,
		"surfaceSoft": p.SurfaceSoft, "border": p.Border,
		"borderSoft": p.BorderSoft, "ink": p.Ink,
		"body": p.Body, "mute": p.Mute,
		"accent": p.Accent, "accentPress": p.AccentPress,
		"onAccent": p.OnAccent, "danger": p.Danger,
		"success": p.Success, "focusRing": p.FocusRing,
	}
}

func (p Palette) complete() bool {
	for _, color := range p.colors() {
		if color == "" {
			return false
		}
	}
	return true
}

// orDefault gives back the stored Palette, or def when it is incomplete.
// Palettes are all-or-nothing: one missing token replaces the whole Palette
// rather than merging, because a v1 document stored a different token set
// that happens to share six names with this one (canvas, ink, body, mute,
// surfaceSoft, focusRing), and merging would keep v1 colours under v2 names.
// No v1 document can be complete, so they all land on the DESIGN.md defaults.
func (p Palette) orDefault(def Palette) Palette {
	if p.complete() {
		return p
	}
	return def
}

// Theme is the Dashboard's stored colours (see CONTEXT.md): the active mode
// plus one Palette per mode. Mode is "auto" until the owner picks a side.
type Theme struct {
	Mode  string  `json:"mode"`
	Light Palette `json:"light"`
	Dark  Palette `json:"dark"`
}

func defaultLightPalette() Palette {
	return Palette{
		Canvas: "#F7F8F9", Surface: "#FFFFFF", SurfaceSoft: "#F2F3F5",
		Border: "#E5E7EB", BorderSoft: "#EFF1F3", Ink: "#0D0F12",
		Body: "#40454D", Mute: "#8B919B", Accent: "#4F6BFF",
		AccentPress: "#3D57EE", OnAccent: "#FFFFFF", Danger: "#E5484D",
		Success: "#2E9E68", FocusRing: "rgba(79,107,255,.45)",
	}
}

func defaultDarkPalette() Palette {
	return Palette{
		Canvas: "#0E1012", Surface: "#17191C", SurfaceSoft: "#121417",
		Border: "#2A2E34", BorderSoft: "#22262B", Ink: "#F3F4F6",
		Body: "#C7CBD1", Mute: "#878D96", Accent: "#7C8DFF",
		AccentPress: "#6A7CFF", OnAccent: "#0E1012", Danger: "#FF6369",
		Success: "#3DD68C", FocusRing: "rgba(124,141,255,.5)",
	}
}

// Migrate upgrades older documents. Palettes saved with the v1 token set are
// replaced by the DESIGN.md defaults rather than carrying their colours over,
// and placeholder "sample" widgets become hello-widget instances.
func (d *Dashboard) Migrate() {
	d.Theme.Light = d.Theme.Light.orDefault(defaultLightPalette())
	d.Theme.Dark = d.Theme.Dark.orDefault(defaultDarkPalette())
	for i := range d.Widgets {
		if d.Widgets[i].Plugin != "sample" {
			continue
		}
		d.Widgets[i].Plugin = "hello-widget"
		d.Widgets[i].Widget = "hello"
		var cfg map[string]string
		if json.Unmarshal(d.Widgets[i].Config, &cfg) == nil && cfg["label"] != "" {
			if raw, err := json.Marshal(map[string]string{"message": cfg["label"]}); err == nil {
				d.Widgets[i].Config = raw
			}
		}
	}
}

// Dashboard is the user's configured set of widgets in an ordered list.
type Dashboard struct {
	Widgets []Widget `json:"widgets"`
	Theme   Theme    `json:"theme"`
}

var validSizes = map[string]bool{"small": true, "medium": true, "large": true}
var validModes = map[string]bool{"light": true, "dark": true, "auto": true}

// DefaultDashboard seeds a fresh panel with base monitoring plus hello
// entries so the panel is useful out of the box via the bundled plugins.
func DefaultDashboard() Dashboard {
	return Dashboard{
		Widgets: []Widget{
			{
				ID:          "stats-1",
				Plugin:      "system-stats",
				Widget:      "system-stats",
				Size:        "medium",
				Enabled:     true,
				PollSeconds: defaultPollSeconds,
			},
			{
				ID:          "hello-1",
				Plugin:      "hello-widget",
				Widget:      "hello",
				Size:        "medium",
				Enabled:     true,
				PollSeconds: defaultPollSeconds,
				Config:      json.RawMessage(`{"message":"Hello from your first Plugin!"}`),
			},
			{
				ID:          "hello-2",
				Plugin:      "hello-widget",
				Widget:      "hello",
				Size:        "small",
				Enabled:     false,
				PollSeconds: defaultPollSeconds,
				Config:      json.RawMessage(`{"message":"Disabled sample — toggle me"}`),
			},
		},
		Theme: Theme{
			Mode:  "auto",
			Light: defaultLightPalette(),
			Dark:  defaultDarkPalette(),
		},
	}
}

// Normalize fills the unset poll intervals before validation. Palette tokens
// are not defaulted here: a save with a missing token is rejected, and only
// documents read from disk are healed (see Migrate).
func (d *Dashboard) Normalize() {
	for i := range d.Widgets {
		if d.Widgets[i].PollSeconds <= 0 {
			d.Widgets[i].PollSeconds = defaultPollSeconds
		}
	}
}

func (d *Dashboard) Validate() error {
	seen := map[string]bool{}
	for i, w := range d.Widgets {
		if w.ID == "" {
			return fmt.Errorf("widget %d: id is required", i)
		}
		if seen[w.ID] {
			return fmt.Errorf("widget %q: duplicate id", w.ID)
		}
		seen[w.ID] = true
		if w.Plugin == "" {
			return fmt.Errorf("widget %q: plugin is required", w.ID)
		}
		if !validSizes[w.Size] {
			return fmt.Errorf("widget %q: size must be small, medium, or large", w.ID)
		}
		if w.PollSeconds <= 0 {
			return fmt.Errorf("widget %q: pollSeconds must be positive", w.ID)
		}
		if len(w.Config) > 0 && !json.Valid(w.Config) {
			return fmt.Errorf("widget %q: config must be valid JSON", w.ID)
		}
	}
	if !validModes[d.Theme.Mode] {
		return errors.New("theme mode must be light or dark")
	}
	for name, palette := range map[string]Palette{
		"light": d.Theme.Light,
		"dark":  d.Theme.Dark,
	} {
		for field, color := range palette.colors() {
			if color == "" {
				return fmt.Errorf("theme %s %s is required", name, field)
			}
		}
	}
	return nil
}
