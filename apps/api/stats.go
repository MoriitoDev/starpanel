package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Stats is the system snapshot served by the built-in system-stats plugin.
// Source names where the numbers came from: "proc" (Linux /proc) or
// "stub" (non-Linux dev placeholder).
type Stats struct {
	Source     string  `json:"source"`
	CPUPercent float64 `json:"cpuPercent"`
	MemTotal   uint64  `json:"memTotalBytes"`
	MemUsed    uint64  `json:"memUsedBytes"`
	DiskTotal  uint64  `json:"diskTotalBytes"`
	DiskUsed   uint64  `json:"diskUsedBytes"`
}

type statsSource interface {
	Snapshot() (Stats, error)
}

func newStatsSource() statsSource {
	if runtime.GOOS == "linux" {
		return &linuxStatsSource{read: readFileDefault, statfs: statfsDefault}
	}
	return stubStatsSource{}
}

// linuxStatsSource parses /proc and statfs with stdlib only. read and
// statfs are injectable so the parsing logic is testable on any OS.
type linuxStatsSource struct {
	mu       sync.Mutex
	read     func(path string) ([]byte, error)
	statfs   func(path string) (*statfsInfo, error)
	lastStat *cpuSample
}

type statfsInfo struct {
	blocks uint64
	bavail uint64
	bsize  uint64
}

func readFileDefault(path string) ([]byte, error) { return os.ReadFile(path) }

type cpuSample struct {
	total uint64
	idle  uint64
}

func (s *linuxStatsSource) Snapshot() (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	memRaw, err := s.read("/proc/meminfo")
	if err != nil {
		return Stats{}, fmt.Errorf("read meminfo: %w", err)
	}
	total, available, err := parseMeminfo(memRaw)
	if err != nil {
		return Stats{}, err
	}

	cpuRaw, err := s.read("/proc/stat")
	if err != nil {
		return Stats{}, fmt.Errorf("read proc stat: %w", err)
	}
	sample, err := parseCPUSample(cpuRaw)
	if err != nil {
		return Stats{}, err
	}
	var cpu float64
	if s.lastStat != nil {
		cpu = cpuPercent(*s.lastStat, sample)
	}
	s.lastStat = &sample

	st, err := s.statfs("/")
	if err != nil {
		return Stats{}, fmt.Errorf("statfs /: %w", err)
	}

	return Stats{
		Source:     "proc",
		CPUPercent: cpu,
		MemTotal:   total,
		MemUsed:    total - available,
		DiskTotal:  st.blocks * st.bsize,
		DiskUsed:   (st.blocks - st.bavail) * st.bsize,
	}, nil
}

// parseMeminfo reads MemTotal and MemAvailable (values are kB).
func parseMeminfo(raw []byte) (total, available uint64, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total, err = parseKb(fields[1])
		case "MemAvailable":
			available, err = parseKb(fields[1])
		}
		if err != nil {
			return 0, 0, err
		}
	}
	if total == 0 || available == 0 {
		return 0, 0, fmt.Errorf("meminfo missing MemTotal or MemAvailable")
	}
	return total, available, nil
}

func parseKb(field string) (uint64, error) {
	kb, err := strconv.ParseUint(field, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse meminfo value %q: %w", field, err)
	}
	return kb * 1024, nil
}

// parseCPUSample reads the aggregate "cpu" line: total jiffies and idle
// jiffies (idle + iowait).
func parseCPUSample(raw []byte) (cpuSample, error) {
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "cpu" {
			continue
		}
		if len(fields) < 5 {
			return cpuSample{}, fmt.Errorf("proc stat cpu line too short: %q", line)
		}
		var vals []uint64
		for _, f := range fields[1:] {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return cpuSample{}, fmt.Errorf("parse cpu value %q: %w", f, err)
			}
			vals = append(vals, v)
		}
		var total uint64
		for _, v := range vals {
			total += v
		}
		idle := vals[3]
		if len(vals) > 4 {
			idle += vals[4] // iowait
		}
		return cpuSample{total: total, idle: idle}, nil
	}
	return cpuSample{}, fmt.Errorf("proc stat has no aggregate cpu line")
}

func cpuPercent(prev, cur cpuSample) float64 {
	dTotal := cur.total - prev.total
	dIdle := cur.idle - prev.idle
	if dTotal == 0 {
		return 0
	}
	busy := float64(dTotal-dIdle) / float64(dTotal) * 100
	if busy < 0 {
		busy = 0
	}
	return busy
}

// stubStatsSource serves plausible fixed values on non-Linux dev so the
// widget demonstrably works without failing (ticket 05).
type stubStatsSource struct{}

const gib = 1024 * 1024 * 1024

func (stubStatsSource) Snapshot() (Stats, error) {
	return Stats{
		Source:     "stub",
		CPUPercent: 25,
		MemTotal:   16 * gib,
		MemUsed:    8 * gib,
		DiskTotal:  512 * gib,
		DiskUsed:   200 * gib,
	}, nil
}

// newStatsHandler serves the built-in system-stats plugin through the
// plugin proxy path: GET <proxy>/stats.
func newStatsHandler(source statsSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such route"})
			return
		}
		stats, err := source.Snapshot()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, stats)
	})
}
