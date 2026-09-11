package dashboard

import (
	"encoding/json"
	"fmt"
)

const DefaultPollSeconds = 10

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

// DefaultTheme is the Theme every Dashboard starts on: the baseline stylesheet
// that ships inside the binary. It is not a file in the themes folder, so it
// is always available and cannot be deleted.
const DefaultTheme = "default"

// ThemeName is the name of the Theme a Dashboard renders with.
//
// It is a plain string in the document, but documents written before Themes
// were stylesheets stored a whole palette object in that field. Reading one is
// therefore a migration rather than a type error: the colours it carried are
// gone, and the Dashboard starts on its default instead of failing to load.
type ThemeName string

func (t *ThemeName) UnmarshalJSON(raw []byte) error {
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		*t = ""
		return nil
	}
	*t = ThemeName(name)
	return nil
}

// migrate upgrades older documents. Palettes saved with the v1 token set are
// replaced by the DESIGN.md defaults rather than carrying their colours over,
// and placeholder "sample" widgets become hello-widget instances. It belongs
// to the store, the only thing that reads documents off disk.
func (d *Dashboard) migrate() {
	if d.Theme == "" {
		d.Theme = DefaultTheme
	}
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
	Theme   ThemeName `json:"theme"`
}

var validSizes = map[string]bool{"small": true, "medium": true, "large": true}

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
				PollSeconds: DefaultPollSeconds,
			},
			{
				ID:          "hello-1",
				Plugin:      "hello-widget",
				Widget:      "hello",
				Size:        "medium",
				Enabled:     true,
				PollSeconds: DefaultPollSeconds,
				Config:      json.RawMessage(`{"message":"Hello from your first Plugin!"}`),
			},
			{
				ID:          "hello-2",
				Plugin:      "hello-widget",
				Widget:      "hello",
				Size:        "small",
				Enabled:     false,
				PollSeconds: DefaultPollSeconds,
				Config:      json.RawMessage(`{"message":"Disabled sample — toggle me"}`),
			},
		},
		Theme: DefaultTheme,
	}
}

// normalize fills the unset poll intervals before validation. Palette tokens
// are not defaulted here: a document with a missing token is rejected rather
// than quietly repainted.
func (d *Dashboard) normalize() {
	for i := range d.Widgets {
		if d.Widgets[i].PollSeconds <= 0 {
			d.Widgets[i].PollSeconds = DefaultPollSeconds
		}
	}
}

// validate reports the first reason a Dashboard cannot be rendered or stored.
func (d *Dashboard) validate() error {
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
	return nil
}
