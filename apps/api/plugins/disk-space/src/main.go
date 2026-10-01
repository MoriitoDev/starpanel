package main

import (
	"encoding/json"
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

// categoryOrder is the order the dialog renders in. It is fixed and explicit,
// because a list that reshuffles between polls is unusable.
var categoryOrder = []CategoryID{
	"docker-images",
	"docker-volumes",
	"rotated-logs",
	"package-caches",
	"temp-files",
	"large-files",
}

// categoryTitles and categoryRules are what a person reads next to the bytes.
var categoryTitles = map[CategoryID]string{
	"docker-images":  "Docker images and cache",
	"docker-volumes": "Docker volumes",
	"rotated-logs":   "Rotated logs",
	"package-caches": "Package caches",
	"temp-files":     "Temporary files",
	"large-files":    "Large files",
}

var categoryRules = map[CategoryID]string{
	"docker-images":  "dangling images, stopped containers, unused networks and the build cache, as Docker itself reports them",
	"docker-volumes": "volumes no container references",
	"rotated-logs":   "/var/log *.log, *.gz and *.1-*.9 older than 30 days, and the systemd journal",
	"package-caches": "apt archives, pip, npm, pnpm, go build and cargo caches older than 30 days",
	"temp-files":     "/tmp and /var/tmp older than 7 days, and /var/crash",
	"large-files":    "over 1 GB under a Scan Root — big, and not necessarily dead",
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
	ID          CategoryID `json:"id"`
	Title       string     `json:"title"`
	Rule        string     `json:"rule"`
	Candidates  int        `json:"candidates"`
	Bytes       uint64     `json:"bytes"`
	Estimate    bool       `json:"estimate"`
	Tickable    bool       `json:"tickable"`
	ReviewOnly  bool       `json:"reviewOnly,omitempty"`
	Capability  Capability `json:"capability"`
	MeasuredAge string     `json:"measuredAge,omitempty"`
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

// placeholder is the capability of a Category whose scanner is not built yet.
func placeholder() Capability {
	return Capability{
		OK:        false,
		Kind:      "not-implemented",
		Words:     "this Category is not measured yet",
		Remedy:    "nothing to do: the Plugin is being built out, ticket by ticket",
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
func (i *index) summary() Summary {
	disks, warnings, err := Disks(readFileDefault, statfsDefault)
	if err != nil {
		disks = []Disk{}
		warnings = append(warnings, "could not read the disks: "+err.Error())
	}
	if disks == nil {
		disks = []Disk{}
	}
	categories := make([]CategorySummary, 0, len(categoryOrder))
	for _, id := range categoryOrder {
		categories = append(categories, CategorySummary{
			ID:         id,
			Title:      categoryTitles[id],
			Rule:       categoryRules[id],
			Tickable:   id != "large-files",
			ReviewOnly: id == "large-files",
			Capability: placeholder(),
		})
	}
	if warnings == nil {
		warnings = []string{}
	}
	return Summary{
		Disks:                 disks,
		ReclaimableBytes:      0,
		ReclaimableIsEstimate: true,
		Categories:            categories,
		Warnings:              warnings,
		IndexAge:              i.age(),
		Source:                sourceName(),
		Version:               version,
	}
}

// sourceName says where the numbers came from, in the one word the card shows.
func sourceName() string {
	return "procfs"
}

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
	})
}

func (s *server) getSummary(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.index.summary())
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
