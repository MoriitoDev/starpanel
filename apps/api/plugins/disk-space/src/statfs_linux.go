//go:build linux

package main

import (
	"os"
	"syscall"
)

// statfsDefault is the one platform-specific call in the Disk listing. Every
// counter is widened to uint64 here so disk.go does its arithmetic without
// knowing which platform's struct it came from. Blocks is unsigned on Linux;
// the int64 counters (Bfree, Bavail) are widened after the kernel has filled
// them, and a negative value is a kernel we do not want to do arithmetic on.
func statfsDefault(path string) (statfsFields, error) {
	var raw syscall.Statfs_t
	if err := syscall.Statfs(path, &raw); err != nil {
		return statfsFields{}, err
	}
	if raw.Bfree < 0 || raw.Bavail < 0 || raw.Blocks < 0 {
		return statfsFields{}, errNegativeStatfs
	}
	return statfsFields{
		blocks: uint64(raw.Blocks),
		bfree:  uint64(raw.Bfree),
		bavail: uint64(raw.Bavail),
		bsize:  uint64(raw.Bsize),
	}, nil
}

// readFileDefault is the real /proc.
func readFileDefault(path string) ([]byte, error) {
	return os.ReadFile(path)
}
