//go:build linux

package main

import (
	"syscall"
)

func statfsDefault(path string) (*statfsInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	return &statfsInfo{blocks: uint64(st.Blocks), bavail: uint64(st.Bavail), bsize: uint64(st.Bsize)}, nil
}
