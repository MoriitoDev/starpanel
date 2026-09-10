package main

import (
	"testing"
)

const meminfoFixture = `MemTotal:       16384 kB
MemFree:         2048 kB
MemAvailable:    8192 kB
Buffers:          512 kB
Cached:          1024 kB
SwapTotal:          0 kB
SwapFree:           0 kB
`

const cpuSample1 = `cpu  100 0 100 200 0 0 0 0 0 0
cpu0 50 0 50 100 0 0 0 0 0 0
`
const cpuSample2 = `cpu  250 0 250 300 0 0 0 0 0 0
cpu0 100 0 100 150 0 0 0 0 0 0
`

func TestParseMeminfo(t *testing.T) {
	total, available, err := parseMeminfo([]byte(meminfoFixture))
	if err != nil {
		t.Fatalf("parseMeminfo: %v", err)
	}
	if total != 16384*1024 {
		t.Errorf("total = %d, want %d", total, 16384*1024)
	}
	if available != 8192*1024 {
		t.Errorf("available = %d, want %d", available, 8192*1024)
	}
}

func TestParseMeminfoRejectsMissingFields(t *testing.T) {
	if _, _, err := parseMeminfo([]byte("SwapTotal: 0 kB\n")); err == nil {
		t.Error("expected error when MemTotal/MemAvailable missing")
	}
}

func TestParseCPUSample(t *testing.T) {
	s, err := parseCPUSample([]byte(cpuSample1))
	if err != nil {
		t.Fatalf("parseCPUSample: %v", err)
	}
	// total = 100+0+100+200, idle = 200
	if s.total != 400 {
		t.Errorf("total = %d, want 400", s.total)
	}
	if s.idle != 200 {
		t.Errorf("idle = %d, want 200", s.idle)
	}
}

func TestCPUPercentBetweenSamples(t *testing.T) {
	s1, err := parseCPUSample([]byte(cpuSample1))
	if err != nil {
		t.Fatalf("sample1: %v", err)
	}
	s2, err := parseCPUSample([]byte(cpuSample2))
	if err != nil {
		t.Fatalf("sample2: %v", err)
	}
	// delta total = 400, delta idle = 100 -> 75% busy
	if got := cpuPercent(s1, s2); got != 75.0 {
		t.Errorf("cpuPercent = %v, want 75.0", got)
	}
}

func TestLinuxSnapshotWithFakes(t *testing.T) {
	files := map[string]string{
		"/proc/meminfo": meminfoFixture,
		"/proc/stat":    cpuSample1,
	}
	src := &linuxStatsSource{
		read: func(path string) ([]byte, error) { return []byte(files[path]), nil },
		statfs: func(path string) (*statfsInfo, error) {
			return &statfsInfo{blocks: 1000, bavail: 250, bsize: 4096}, nil
		},
	}

	first, err := src.Snapshot()
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if first.Source != "proc" {
		t.Errorf("source = %q, want proc", first.Source)
	}
	if first.MemTotal != 16384*1024 || first.MemUsed != (16384-8192)*1024 {
		t.Errorf("memory wrong: %+v", first)
	}
	if first.DiskTotal != 1000*4096 || first.DiskUsed != (1000-250)*4096 {
		t.Errorf("disk wrong: %+v", first)
	}
	if first.CPUPercent != 0 {
		t.Errorf("first sample cpuPercent = %v, want 0 (no previous sample)", first.CPUPercent)
	}

	// Second snapshot computes the busy percentage against the previous one.
	files["/proc/stat"] = cpuSample2
	second, err := src.Snapshot()
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if second.CPUPercent != 75.0 {
		t.Errorf("second cpuPercent = %v, want 75.0", second.CPUPercent)
	}
}

func TestStubSnapshotIsDeterministicAndMarked(t *testing.T) {
	a, err := stubStatsSource{}.Snapshot()
	if err != nil {
		t.Fatalf("stub snapshot: %v", err)
	}
	b, _ := stubStatsSource{}.Snapshot()
	if a != b {
		t.Errorf("stub not deterministic: %+v vs %+v", a, b)
	}
	if a.Source != "stub" {
		t.Errorf("source = %q, want stub", a.Source)
	}
	if a.MemTotal == 0 || a.DiskTotal == 0 {
		t.Errorf("stub should carry plausible values: %+v", a)
	}
}
