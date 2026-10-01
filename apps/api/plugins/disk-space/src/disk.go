package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// errNegativeStatfs is a kernel lying to us: bfree and bavail are int64 on
// Linux and a negative counter means the values are not to be trusted.
var errNegativeStatfs = errors.New("statfs reported a negative block count")

// Disk is a mounted filesystem with its own capacity, named by its mount point.
// The vocabulary is CONTEXT.md: a machine has several Disks and none of them is
// "the disk".
type Disk struct {
	// Name is what a person calls it: the mount point on Linux, the drive
	// letter on Windows.
	Name string `json:"name"`
	// Label is the volume's own label where the platform has one. Linux
	// mounts carry none, so it is empty there.
	Label string `json:"label"`
	// FS is the filesystem type: ext4, xfs, NTFS.
	FS string `json:"fs"`
	// Mount is where it is mounted.
	Mount       string  `json:"mount"`
	TotalBytes  uint64  `json:"totalBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	FreeBytes   uint64  `json:"freeBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

// statfsFields is what one statfs call has to hand back for the arithmetic in
// Disks to be portable: the counters are the kernel's, the multiplication is
// ours, and no platform's struct leaks past this type.
type statfsFields struct {
	blocks uint64
	bfree  uint64
	bavail uint64
	bsize  uint64
}

// pseudoFilesystems are mounted filesystems that are not storage. Listing them
// as Disks would be worse than listing none: a person reads a bar and believes
// it.
//
// overlay is deliberately absent: inside the container this Plugin usually runs
// in, `/` is an overlay and it is the Disk the panel has.
var pseudoFilesystems = map[string]bool{
	"autofs":      true,
	"binfmt_misc": true,
	"bpf":         true,
	"cgroup":      true,
	"cgroup2":     true,
	"configfs":    true,
	"debugfs":     true,
	"devpts":      true,
	"devtmpfs":    true,
	"efivarfs":    true,
	"fusectl":     true,
	"hugetlbfs":   true,
	"mqueue":      true,
	"nsfs":        true,
	"proc":        true,
	"pstore":      true,
	"ramfs":       true,
	"securityfs":  true,
	"selinuxfs":   true,
	"squashfs":    true,
	"sysfs":       true,
	"tmpfs":       true,
	"tracefs":     true,
}

// mountEntry is one line of /proc/mounts, decoded.
type mountEntry struct {
	device string
	mount  string
	fs     string
}

// parseMounts reads the kernel's mount table. The format is
// `device mountpoint fstype options dump pass`, with the two escapes the kernel
// documents (`\040` for a space, `\011` for a tab) in the first two fields.
func parseMounts(raw string) []mountEntry {
	var entries []mountEntry
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		entries = append(entries, mountEntry{
			device: unescapeMount(fields[0]),
			mount:  unescapeMount(fields[1]),
			fs:     fields[2],
		})
	}
	return entries
}

// unescapeMount undoes the octal escapes /proc/mounts uses, so a path with a
// space is reported as the path it is.
func unescapeMount(field string) string {
	if !strings.Contains(field, `\`) {
		return field
	}
	var out strings.Builder
	for i := 0; i < len(field); i++ {
		if field[i] == '\\' && i+3 < len(field) && isOctal(field[i+1:i+4]) {
			out.WriteByte(byte((field[i+1]-'0')<<6 | (field[i+2]-'0')<<3 | (field[i+3] - '0')))
			i += 3
			continue
		}
		out.WriteByte(field[i])
	}
	return out.String()
}

func isOctal(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '7' {
			return false
		}
	}
	return true
}

// Disks answers every mounted filesystem worth showing, in the order a person
// reads them: by mount point.
//
// read is injectable so the parsing is testable without a real /proc, and
// statfs is the one platform-specific call. A Disk whose statfs fails is
// omitted rather than reported as zero: a zero-capacity Disk would make the
// card say a machine has no space at all.
//
// roots are the Scan Roots the owner declared. They matter here for exactly one
// case: a tmpfs at a declared root is the Disk the owner asked about — a panel
// whose /tmp is a tmpfs and who scans /tmp wants to see it — while every other
// tmpfs is not storage and listing it would put a bar on a card that lies.
func Disks(read func(string) ([]byte, error), statfs func(string) (statfsFields, error), roots []string) ([]Disk, []string, error) {
	raw, err := read("/proc/mounts")
	if err != nil {
		return nil, nil, err
	}
	entries := parseMounts(string(raw))

	// One device can be mounted in several places (binds, chroots). Keeping the
	// shortest mount point per device is what stops the same capacity being
	// reported three times under three paths.
	best := map[string]mountEntry{}
	for _, entry := range entries {
		if !worthShowing(entry, roots) {
			continue
		}
		current, seen := best[entry.device]
		if !seen || isBetterMount(entry.mount, current.mount) {
			best[entry.device] = entry
		}
	}

	chosen := make([]mountEntry, 0, len(best))
	for _, entry := range best {
		chosen = append(chosen, entry)
	}
	sort.Slice(chosen, func(i, j int) bool {
		if chosen[i].mount != chosen[j].mount {
			return chosen[i].mount < chosen[j].mount
		}
		return chosen[i].device < chosen[j].device
	})

	disks := make([]Disk, 0, len(chosen))
	var warnings []string
	for _, entry := range chosen {
		fields, err := statfs(entry.mount)
		if err != nil {
			warnings = append(warnings, "could not read "+entry.mount+": "+err.Error())
			continue
		}
		disks = append(disks, newDisk(entry, fields))
	}
	return disks, warnings, nil
}

// worthShowing is the pseudo-filesystem filter, with the one exception the
// owner can make: a tmpfs mounted at a declared Scan Root is the Disk they
// asked about, so it is shown and everything else that is not storage is not.
func worthShowing(entry mountEntry, roots []string) bool {
	if !pseudoFilesystems[entry.fs] {
		return true
	}
	if entry.fs != "tmpfs" {
		return false
	}
	for _, root := range roots {
		if cleanPath(root) == cleanPath(entry.mount) {
			return true
		}
	}
	return false
}

// cleanPath normalises a path for the comparison above: no trailing slash, and
// always absolute. It is deliberately not filepath.Clean, which on Windows
// would rewrite a Unix path's separators.
func cleanPath(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

// isBetterMount prefers the shorter path, and the alphabetically first on a
// tie, so the answer does not depend on the order the kernel listed mounts in.
func isBetterMount(candidate, current string) bool {
	if len(candidate) != len(current) {
		return len(candidate) < len(current)
	}
	return candidate < current
}

// newDisk does the arithmetic. freeBytes comes from bavail, never bfree: the
// difference between them is the root reserve, and calling bfree "free" tells
// an owner they have space they cannot use.
func newDisk(entry mountEntry, fields statfsFields) Disk {
	total := fields.blocks * fields.bsize
	free := fields.bavail * fields.bsize
	used := (fields.blocks - fields.bfree) * fields.bsize
	disk := Disk{
		Name:       entry.mount,
		Label:      mountLabel(entry.mount),
		FS:         entry.fs,
		Mount:      entry.mount,
		TotalBytes: total,
		UsedBytes:  used,
		FreeBytes:  free,
	}
	if total > 0 {
		disk.UsedPercent = round1(float64(used) / float64(total) * 100)
	}
	return disk
}

// mountLabel is the word a person would use for a mount point, for the case
// where a platform has no real volume label to show.
func mountLabel(mount string) string {
	switch mount {
	case "/":
		return "root"
	case "/home":
		return "home"
	case "/var":
		return "var"
	case "/tmp":
		return "tmp"
	case "/boot":
		return "boot"
	}
	trimmed := strings.Trim(mount, "/")
	if trimmed == "" {
		return ""
	}
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[index+1:]
	}
	return trimmed
}

// round1 keeps one decimal, which is all a percentage bar can honestly show.
func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

// humanBytes is the same shape the card uses for its figures, in Go: the rule
// a Category prints has to quote the threshold the same way the owner set it.
func humanBytes(bytes uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	rounded := round1(value)
	if rounded == float64(int64(rounded)) {
		return fmt.Sprintf("%d %s", int64(rounded), units[unit])
	}
	return fmt.Sprintf("%.1f %s", rounded, units[unit])
}
