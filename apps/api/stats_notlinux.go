//go:build !linux

package main

import "errors"

// statfsDefault is never called on non-Linux: newStatsSource picks the
// stub source there. It exists so the package compiles anywhere.
func statfsDefault(path string) (*statfsInfo, error) {
	return nil, errors.New("statfs requires linux")
}
