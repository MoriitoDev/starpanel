package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// version is stamped by the build script and read by GET /health.
var version = "0.1.0"

// CategoryID names a Category: a class of removable waste this Plugin knows how
// to find and the rules it finds it by (CONTEXT.md).
type CategoryID string

// categoryMeta is one Category as the card and the dialog read it. The order of
// this slice is the order they render in — fixed and explicit, because a list
// that reshuffles between polls is unusable. reviewOnly is the one fact that
// decides both flags on the wire: a Category that only ever recommends never
// offers a tick box (D10). The rule is a function of the settings because the
// sentence a person reads quotes a threshold, and a threshold baked into the
// text is a sentence the owner's config can contradict.
type categoryMeta struct {
	id         CategoryID
	title      string
	rule       func(Settings) string
	reviewOnly bool
}

var categories = []categoryMeta{
	{
		id:    "docker-images",
		title: "Docker images and cache",
		rule: func(Settings) string {
			return "dangling images, stopped containers, unused networks and the build cache, as Docker itself reports them"
		},
	},
	{
		id:    "docker-volumes",
		title: "Docker volumes",
		rule: func(Settings) string {
			return "volumes no container references — a volume is data, not junk"
		},
	},
	{
		id:    "rotated-logs",
		title: "Rotated logs",
		rule: func(s Settings) string {
			return fmt.Sprintf("/var/log *.log, *.gz and *.1-*.9 older than %d days, and the systemd journal", s.LogDays)
		},
	},
	{
		id:    "package-caches",
		title: "Package caches",
		rule: func(s Settings) string {
			return fmt.Sprintf("apt archives, pip, npm, pnpm, go build and cargo caches older than %d days", s.CacheDays)
		},
	},
	{
		id:    "temp-files",
		title: "Temporary files",
		rule: func(s Settings) string {
			return fmt.Sprintf("/tmp and /var/tmp older than %d days, and /var/crash", s.TempDays)
		},
	},
	{
		id:         "large-files",
		title:      "Large files",
		reviewOnly: true,
		rule: func(s Settings) string {
			return fmt.Sprintf("over %s under a Scan Root — big, and not necessarily dead", humanBytes(s.LargeFileBytes))
		},
	},
}

// Settings is what the widget's config came to: the paths and thresholds every
// Category measures by, with the documented defaults where the config said
// nothing usable. Ticket 09 owns the validation and the notes it produces; this
// is the seam it fills.
type Settings struct {
	ScanRoots      []string `json:"scanRoots"`
	LogDays        int      `json:"logDays"`
	CacheDays      int      `json:"cacheDays"`
	TempDays       int      `json:"tempDays"`
	LargeFileBytes uint64   `json:"largeFileBytes"`
	WarnPercent    int      `json:"warnPercent"`
}

// defaultSettings is spec §8's list. ScanRoots is the places waste actually
// lives rather than "/": a default that points the hot index at every Disk is a
// default nobody intended (D14).
func defaultSettings() Settings {
	return Settings{
		ScanRoots:      []string{"/var/log", "/var/cache", "/tmp", "/var/tmp", "/var/lib/docker", "/var/crash"},
		LogDays:        30,
		CacheDays:      30,
		TempDays:       7,
		LargeFileBytes: 1073741824,
		WarnPercent:    90,
	}
}

// Capability is whether this process may act on a Candidate at all, on a
// Category and on a Candidate with the same shape, so the dialog has one rule.
type Capability struct {
	OK bool `json:"ok"`
	// Kind is ok, fs-permission, tool-missing, socket-missing,
	// socket-permission, read-only-filesystem, not-found, not-a-directory or
	// out-of-declared-root.
	Kind string `json:"kind,omitempty"`
	// Errno is the errno when there is one: EACCES, EROFS, ENOENT.
	Errno string `json:"errno,omitempty"`
	// Attempted is what was tried, in words: "deleting /var/log/syslog".
	Attempted string `json:"attempted,omitempty"`
	// Words is the first line of the row's tooltip, always containing the errno
	// the way a person says it.
	Words string `json:"words,omitempty"`
	// Remedy is the second line: the concrete thing the owner can change.
	Remedy string `json:"remedy,omitempty"`
}

// CategorySummary is one Category on the card's poll: how many Candidates it
// found and how much they occupy, with no Candidate list attached.
type CategorySummary struct {
	ID         CategoryID `json:"id"`
	Title      string     `json:"title"`
	Rule       string     `json:"rule"`
	Candidates int        `json:"candidates"`
	Bytes      uint64     `json:"bytes"`
	Estimate   bool       `json:"estimate"`
	Tickable   bool       `json:"tickable"`
	ReviewOnly bool       `json:"reviewOnly,omitempty"`
	Capability Capability `json:"capability"`
}

// Summary is what the card polls. It is cheap by construction: statfs per Disk
// plus the hot index, never a walk.
type Summary struct {
	Disks                 []Disk            `json:"disks"`
	ReclaimableBytes      uint64            `json:"reclaimableBytes"`
	ReclaimableIsEstimate bool              `json:"reclaimableIsEstimate"`
	Categories            []CategorySummary `json:"categories"`
	Warnings              []string          `json:"warnings"`
	IndexAge              string            `json:"indexAge"`
	Source                string            `json:"source"`
	Version               string            `json:"version"`
}

// ok is the capability of something this process may act on. It carries
// nothing else, which is what makes `capability.ok` the one field a reader has
// to check first.
func ok() Capability { return Capability{OK: true} }

// unimplemented is the capability of a Category whose scanner does not exist
// yet. It is honest rather than silent: nothing is tickable, and the Category
// says why in the words the row would show.
func unimplemented() Capability {
	return Capability{
		OK:     false,
		Kind:   "not-implemented",
		Words:  "this Category is not measured yet",
		Remedy: "no action needed",
	}
}

// index is the hot index over the declared Scan Roots. Ticket 03 fills it with
// real scanners; until then it knows the Categories and reports zeros, so the
// wire shape the dialog reads is already the final one.
type index struct {
	measuredAt time.Time
}

func newIndex() *index {
	return &index{measuredAt: time.Now()}
}

func (i *index) age() string {
	return shortDuration(time.Since(i.measuredAt))
}

// summary assembles the card's answer. Nothing here fails as a block: a Disk
// listing this process cannot read is a warning beside the rest of the answer,
// because the card still has Categories and an age to show, and a card that
// says "could not read the disks" is more use than a card that says 500.
func (i *index) summary(settings Settings) Summary {
	disks, warnings, err := Disks(readFileDefault, statfsDefault, settings.ScanRoots)
	if err != nil {
		disks = []Disk{}
		warnings = append(warnings, "could not read the disks: "+err.Error())
	}
	if disks == nil {
		disks = []Disk{}
	}
	summaries := make([]CategorySummary, 0, len(categories))
	for _, meta := range categories {
		summaries = append(summaries, CategorySummary{
			ID:         meta.id,
			Title:      meta.title,
			Rule:       meta.rule(settings),
			Tickable:   !meta.reviewOnly,
			ReviewOnly: meta.reviewOnly,
			Capability: unimplemented(),
		})
	}
	if warnings == nil {
		warnings = []string{}
	}
	return Summary{
		Disks:                 disks,
		ReclaimableBytes:      0,
		ReclaimableIsEstimate: true,
		Categories:            summaries,
		Warnings:              warnings,
		IndexAge:              i.age(),
		Source:                sourceName(),
		Version:               version,
	}
}

// sourceName says where the numbers came from, in the one word the card's
// footer shows. It is per-platform because the answer is: a build that cannot
// read /proc must not label its output "procfs".

// shortDuration is the age as a person reads it: seconds, then minutes, then
// hours. "1m2.5s" is not an answer to "how old is this".
func shortDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return strconv.Itoa(int(d.Seconds())) + "s"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	default:
		return strconv.Itoa(int(d.Hours())) + "h"
	}
}

type server struct {
	index *index
}

func newServer() *server {
	return &server{index: newIndex()}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /summary", s.getSummary)
	return mux
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"version":  version,
		"indexAge": s.index.age(),
		// docker is spec §3's field: whether the CLI this Plugin needs for two
		// of its Categories is there. It is measured, never installed.
		"docker": dockerState(),
	})
}

func (s *server) getSummary(w http.ResponseWriter, r *http.Request) {
	settings, _ := settingsFrom(r.URL.Query().Get("config"))
	writeJSON(w, http.StatusOK, s.index.summary(settings))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError answers with the shape core speaks: {"error": "..."} is part of
// the frozen interface, and a widget shows the message it finds there.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// listenAddr resolves the address core handed over. Core allocates a free port
// and passes it as STAR_PANEL_PORT; a bare run without it picks any free port,
// which is what makes the binary testable and runnable by hand.
func listenAddr() string {
	port := strings.TrimSpace(os.Getenv("STAR_PANEL_PORT"))
	if port == "" {
		return "127.0.0.1:0"
	}
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("STAR_PANEL_PORT is not a port: %q", port)
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func main() {
	addr := listenAddr()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", addr, err)
	}
	// The bound address is logged because core's startup wait is a TCP dial:
	// saying it out loud is what makes a failed handshake diagnosable.
	log.Printf("disk-space %s listening on %s", version, listener.Addr())
	if err := http.Serve(listener, newServer().routes()); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
