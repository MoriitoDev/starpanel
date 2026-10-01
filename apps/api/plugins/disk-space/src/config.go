package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// settingsFrom turns the widget's config into the Settings every Category
// measures by, and the notes the owner should read about what it could not use.
//
// The config reaches the backend only because the widget sends it on each
// request — core hands a subprocess nothing but its port and its name — so it
// arrives as an opaque string here and is validated per request. It is read
// defensively on purpose: this is free-form JSON an owner edited by hand, and a
// typo must cost a default and a note, never a 4xx.
//
// Ticket 09 owns the full validation, the clamping and the wording of the notes;
// this is the seam it fills, with the guards that would otherwise let a config
// turn a safe plugin into a dangerous one.
func settingsFrom(configQuery string) (Settings, []string) {
	settings := defaultSettings()
	if trimmed := strings.TrimSpace(configQuery); trimmed != "" {
		raw, err := url.QueryUnescape(trimmed)
		if err != nil {
			return settings, []string{"the config could not be read; using the defaults"}
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return settings, []string{"the config could not be read; using the defaults"}
		}
		notes := applySettings(&settings, decoded)
		return settings, notes
	}
	return settings, nil
}

// applySettings reads the keys it understands and ignores the rest, one key at a
// time: a wrong type in one key must not throw away the others.
func applySettings(settings *Settings, decoded map[string]json.RawMessage) []string {
	var notes []string

	if raw, ok := decoded["scanRoots"]; ok {
		var roots []string
		if err := json.Unmarshal(raw, &roots); err != nil {
			notes = append(notes, "scanRoots was not a list of paths; using the defaults")
		} else {
			var usable []string
			for _, root := range roots {
				if !isAbsolutePath(root) {
					notes = append(notes, fmt.Sprintf("scanRoots entry %q is not an absolute path; it was dropped", root))
					continue
				}
				usable = append(usable, root)
			}
			if len(usable) > 0 {
				settings.ScanRoots = usable
			}
		}
	}

	notes = appendInt(notes, decoded, "logDays", &settings.LogDays, 1)
	notes = appendInt(notes, decoded, "cacheDays", &settings.CacheDays, 1)
	notes = appendInt(notes, decoded, "tempDays", &settings.TempDays, 1)
	notes = appendInt(notes, decoded, "warnPercent", &settings.WarnPercent, 1)
	if settings.WarnPercent > 100 {
		notes = append(notes, "warnPercent was over 100; using 100")
		settings.WarnPercent = 100
	}

	if raw, ok := decoded["largeFileBytes"]; ok {
		var bytes uint64
		if err := json.Unmarshal(raw, &bytes); err != nil {
			notes = append(notes, "largeFileBytes was not a number of bytes; using the default")
		} else if bytes < 1024 {
			// A floor below a kilobyte would make every file a Candidate, and
			// the whole point of this Category is that it only speaks about
			// files big enough to be worth a person's attention (D10).
			notes = append(notes, "largeFileBytes was under 1 KB; using the default")
		} else {
			settings.LargeFileBytes = bytes
		}
	}

	return notes
}

// appendInt reads one integer key, refusing anything that would make a
// threshold meaningless: a tempDays of 0 would make every file in /tmp a
// Candidate, which is the exact opposite of a conservative default.
func appendInt(notes []string, decoded map[string]json.RawMessage, key string, target *int, floor int) []string {
	raw, ok := decoded[key]
	if !ok {
		return notes
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return append(notes, fmt.Sprintf("%s was not a number; using %d", key, *target))
	}
	if value < floor {
		return append(notes, fmt.Sprintf("%s was %d; a value under %d is not meaningful, so %d was used", key, value, floor, *target))
	}
	*target = value
	return notes
}

// isAbsolutePath is the check for a Scan Root. It is deliberately the Unix rule
// and not filepath.IsAbs, because a Scan Root is a Linux path whatever platform
// this happened to compile on: the Plugin measures a Linux machine.
func isAbsolutePath(path string) bool {
	if !strings.HasPrefix(path, "/") || path == "/" {
		return strings.HasPrefix(path, "/")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

// dockerState says whether the CLI two Categories need is here. It is measured,
// never installed: the panel diagnoses and the owner decides. It asks the
// filesystem rather than running the CLI, because a poll must never wait on a
// subprocess.
func dockerState() string {
	if _, err := exec.LookPath("docker"); err != nil {
		return "missing"
	}
	return "ok"
}
