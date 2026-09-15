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
	ID     string `json:"id"`
	Plugin string `json:"plugin"`
	Widget string `json:"widget,omitempty"`
	Span
	Enabled     bool            `json:"enabled"`
	PollSeconds int             `json:"pollSeconds"`
	Config      json.RawMessage `json:"config,omitempty"`

	// Size is what a Widget declared before Spans (ADR-0008). It is read for
	// two reasons — a document written by an older panel still has to load,
	// and a client that keeps sending it has to be told what to send instead —
	// and it is never written back out.
	Size string `json:"size,omitempty"`
}

// Span is how much of the grid a Widget fills: columns wide and rows tall
// (CONTEXT.md). It is embedded, so a document carries `w` and `h` beside the
// Widget's other fields, and the bounds travel with the numbers they bound.
type Span struct {
	W int `json:"w"`
	H int `json:"h"`
}

// A Widget fills between one and twelve of the grid's columns, and between one
// and twelve rows (DESIGN.md §6). Outside that there is no grid left to fill.
const (
	MinSpan = 1
	MaxSpan = 12
)

func (s Span) valid() bool {
	return within(s.W) && within(s.H)
}

func within(n int) bool { return n >= MinSpan && n <= MaxSpan }

// legacySpans is what the three sizes a Widget used to declare became: four,
// six and twelve of a twelve-column grid, one row tall (ADR-0008).
var legacySpans = map[string]Span{
	"small":  {W: 4, H: 1},
	"medium": {W: 6, H: 1},
	"large":  {W: 12, H: 1},
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

// migrate upgrades older documents: palettes saved with the v1 token set are
// replaced by the DESIGN.md defaults rather than carrying their colours over,
// placeholder "sample" widgets become hello-widget instances, and a Widget
// that still declares one of the three old sizes gets the Span those became
// (ADR-0008). It belongs to the store, the only thing that reads documents off
// disk — which is also why a document that arrives over the API is never
// migrated: a client that sends a size is answered, not corrected.
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
	for i := range d.Widgets {
		w := &d.Widgets[i]
		if w.Size == "" || w.Span.valid() {
			continue
		}
		if span, ok := legacySpans[w.Size]; ok {
			w.Span = span
		}
	}
}

// Dashboard is the user's configured set of widgets in an ordered list.
type Dashboard struct {
	Widgets []Widget  `json:"widgets"`
	Theme   ThemeName `json:"theme"`
}

// DefaultDashboard seeds a fresh panel with base monitoring plus hello
// entries so the panel is useful out of the box via the bundled plugins.
func DefaultDashboard() Dashboard {
	return Dashboard{
		Widgets: []Widget{
			{
				ID:          "stats-1",
				Plugin:      "system-stats",
				Widget:      "system-stats",
				Span:        Span{W: 6, H: 1},
				Enabled:     true,
				PollSeconds: DefaultPollSeconds,
			},
			{
				ID:          "hello-1",
				Plugin:      "hello-widget",
				Widget:      "hello",
				Span:        Span{W: 6, H: 1},
				Enabled:     true,
				PollSeconds: DefaultPollSeconds,
				Config:      json.RawMessage(`{"message":"Hello from your first Plugin!"}`),
			},
			{
				ID:          "hello-2",
				Plugin:      "hello-widget",
				Widget:      "hello",
				Span:        Span{W: 4, H: 1},
				Enabled:     false,
				PollSeconds: DefaultPollSeconds,
				Config:      json.RawMessage(`{"message":"Disabled sample — toggle me"}`),
			},
		},
		Theme: DefaultTheme,
	}
}

// forgetRetiredFields drops what the document no longer carries, so neither a
// stored document nor an answer to a caller ever advertises a size again.
func (d *Dashboard) forgetRetiredFields() {
	for i := range d.Widgets {
		d.Widgets[i].Size = ""
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
		if !w.Span.valid() {
			return spanError(w)
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

// spanError names the side of the Span that is out of range, and — when the
// Widget still carries the size Spans replaced — says what to send instead,
// because that is the shape of a client that has not heard of Spans (ADR-0008).
func spanError(w Widget) error {
	if w.Size != "" {
		return fmt.Errorf(
			"widget %q: a Dashboard has no size any more; send w and h (each between %d and %d) instead",
			w.ID, MinSpan, MaxSpan)
	}
	if !within(w.W) {
		return fmt.Errorf("widget %q: w must be between %d and %d", w.ID, MinSpan, MaxSpan)
	}
	return fmt.Errorf("widget %q: h must be between %d and %d", w.ID, MinSpan, MaxSpan)
}
