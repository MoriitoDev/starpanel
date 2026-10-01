package main

import (
	"errors"
	"testing"
)

// procMounts is a /proc/mounts shaped like a real container host's: the two
// escapes the kernel documents, pseudo-filesystems, and one device mounted
// twice.
const procMounts = `/dev/nvme0n1p2 / ext4 rw,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /tmp tmpfs rw,nosuid,nodev 0 0
/dev/nvme0n1p1 /boot/efi vfat rw,relatime 0 0
/dev/sdb1 /srv/media ext4 rw,relatime 0 0
/dev/sdb1 /srv/media/photos ext4 rw,relatime 0 0
/dev/sdc1 /mnt/with\040space xfs rw,relatime 0 0
`

func reading(raw string) func(string) ([]byte, error) {
	return func(string) ([]byte, error) { return []byte(raw), nil }
}

// answering returns fixed counters per mount point, so a test can say what the
// kernel says without a real filesystem.
func answering(byMount map[string]statfsFields) func(string) (statfsFields, error) {
	return func(path string) (statfsFields, error) {
		fields, ok := byMount[path]
		if !ok {
			return statfsFields{}, errors.New("no such mount: " + path)
		}
		return fields, nil
	}
}

func TestParseMountsReadsTheKernelTable(t *testing.T) {
	entries := parseMounts(procMounts)
	if len(entries) != 8 {
		t.Fatalf("parsed %d mounts, want 8", len(entries))
	}
	if entries[0].device != "/dev/nvme0n1p2" || entries[0].mount != "/" || entries[0].fs != "ext4" {
		t.Errorf("first entry is %+v", entries[0])
	}
}

func TestParseMountsUndoesTheKernelsOctalEscapes(t *testing.T) {
	entries := parseMounts(procMounts)
	last := entries[len(entries)-1]
	if last.mount != "/mnt/with space" {
		t.Errorf("mount point is %q, want %q", last.mount, "/mnt/with space")
	}
}

func TestDisksLeavesOutWhatIsNotStorage(t *testing.T) {
	disks, _, err := Disks(reading(procMounts), answering(map[string]statfsFields{
		"/":     {blocks: 100, bfree: 40, bavail: 30, bsize: 1024},
		"/tmp":  {blocks: 10, bfree: 10, bavail: 10, bsize: 1024},
		"/proc": {blocks: 1, bfree: 1, bavail: 1, bsize: 1},
		"/sys":  {blocks: 1, bfree: 1, bavail: 1, bsize: 1},
	}), nil)
	if err != nil {
		t.Fatalf("Disks: %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("listed %d Disks, want 1: %+v", len(disks), disks)
	}
	if disks[0].Mount != "/" {
		t.Errorf("listed %q, want /", disks[0].Mount)
	}
}

// A tmpfs at a declared Scan Root is the Disk the owner asked about, and
// filtering it would leave the card unable to explain the very path the Plugin
// is measuring.
func TestDisksShowsATmpfsAtADeclaredScanRoot(t *testing.T) {
	statfs := answering(map[string]statfsFields{
		"/":    {blocks: 100, bfree: 40, bavail: 30, bsize: 1024},
		"/tmp": {blocks: 10, bfree: 4, bavail: 4, bsize: 1024},
	})
	disks, _, err := Disks(reading(procMounts), statfs, []string{"/tmp"})
	if err != nil {
		t.Fatalf("Disks: %v", err)
	}
	var found bool
	for _, disk := range disks {
		if disk.Mount == "/tmp" {
			found = true
			if disk.FS != "tmpfs" {
				t.Errorf("the /tmp Disk reports filesystem %q", disk.FS)
			}
		}
	}
	if !found {
		t.Errorf("a tmpfs at a declared Scan Root was filtered out: %+v", disks)
	}
}

// And a tmpfs nobody declared stays out, because it is not storage.
func TestDisksStillLeavesOutAnUndeclaredTmpfs(t *testing.T) {
	disks, _, err := Disks(reading(procMounts), answering(map[string]statfsFields{
		"/":    {blocks: 100, bfree: 40, bavail: 30, bsize: 1024},
		"/tmp": {blocks: 10, bfree: 4, bavail: 4, bsize: 1024},
	}), []string{"/var/log"})
	if err != nil {
		t.Fatalf("Disks: %v", err)
	}
	for _, disk := range disks {
		if disk.Mount == "/tmp" {
			t.Errorf("an undeclared tmpfs was listed: %+v", disk)
		}
	}
}

// A device mounted twice is one Disk. Two bars for one capacity is how a person
// learns not to trust the panel.
func TestDisksReportsADeviceOnce(t *testing.T) {
	disks, _, err := Disks(reading(procMounts), answering(map[string]statfsFields{
		"/":                {blocks: 100, bfree: 40, bavail: 30, bsize: 1024},
		"/boot/efi":        {blocks: 10, bfree: 5, bavail: 5, bsize: 1024},
		"/srv/media":       {blocks: 200, bfree: 100, bavail: 90, bsize: 1024},
		"/srv/media/photo": {blocks: 200, bfree: 100, bavail: 90, bsize: 1024},
		"/mnt/with space":  {blocks: 50, bfree: 25, bavail: 25, bsize: 1024},
	}), nil)
	if err != nil {
		t.Fatalf("Disks: %v", err)
	}
	var media []string
	for _, disk := range disks {
		if disk.FS == "ext4" && disk.Mount != "/" {
			media = append(media, disk.Mount)
		}
	}
	if len(media) != 1 || media[0] != "/srv/media" {
		t.Errorf("the twice-mounted device produced %v, want [/srv/media]", media)
	}
}

// freeBytes comes from bavail, not bfree: the difference is the root reserve,
// and calling bfree "free" promises space an owner cannot have.
func TestDisksCountsFreeSpaceAsAvailableSpace(t *testing.T) {
	disks, _, err := Disks(
		reading("/dev/sda1 / ext4 rw 0 0\n"),
		answering(map[string]statfsFields{
			"/": {blocks: 1000, bfree: 500, bavail: 400, bsize: 1024},
		}),
		nil,
	)
	if err != nil {
		t.Fatalf("Disks: %v", err)
	}
	disk := disks[0]
	if disk.TotalBytes != 1024*1000 {
		t.Errorf("total is %d, want %d", disk.TotalBytes, 1024*1000)
	}
	if disk.FreeBytes != 1024*400 {
		t.Errorf("free is %d, want %d (bavail, not bfree)", disk.FreeBytes, 1024*400)
	}
	if disk.UsedBytes != 1024*500 {
		t.Errorf("used is %d, want %d", disk.UsedBytes, 1024*500)
	}
	if disk.UsedPercent != 50 {
		t.Errorf("usedPercent is %v, want 50", disk.UsedPercent)
	}
}

// One unreadable Disk must not take the listing down with it; it is reported
// and the rest are still measured.
func TestDisksSurvivesAStatfsFailure(t *testing.T) {
	disks, warnings, err := Disks(reading(procMounts), func(path string) (statfsFields, error) {
		if path == "/" {
			return statfsFields{}, errors.New("permission denied")
		}
		return statfsFields{blocks: 100, bfree: 50, bavail: 40, bsize: 1024}, nil
	}, nil)
	if err != nil {
		t.Fatalf("Disks returned an error instead of a warning: %v", err)
	}
	for _, disk := range disks {
		if disk.Mount == "/" {
			t.Error("a Disk whose statfs failed was reported anyway")
		}
	}
	if len(warnings) == 0 {
		t.Error("the failure was not reported in warnings")
	}
	if len(disks) == 0 {
		t.Error("one failure took the whole listing down")
	}
}

// A machine with nothing worth showing is not an error.
func TestDisksOnAnEmptyTableIsEmpty(t *testing.T) {
	disks, warnings, err := Disks(reading(""), answering(nil), nil)
	if err != nil {
		t.Fatalf("Disks: %v", err)
	}
	if len(disks) != 0 || len(warnings) != 0 {
		t.Errorf("got %d Disks and %d warnings, want none of either", len(disks), len(warnings))
	}
}

func TestMountLabelNamesTheCommonOnes(t *testing.T) {
	cases := map[string]string{
		"/":                  "root",
		"/home":              "home",
		"/srv/media":         "media",
		"/mnt/with space":    "with space",
		"/var/lib/docker":    "docker",
		"/deeply/nested/xfs": "xfs",
	}
	for mount, want := range cases {
		if got := mountLabel(mount); got != want {
			t.Errorf("mountLabel(%q) = %q, want %q", mount, got, want)
		}
	}
}
