//go:build !linux

package main

import "errors"

// This Plugin promises linux/amd64 and nothing else. The non-Linux build exists
// so the package compiles — and so `go vet` and the unit tests run on a
// developer's machine — and it says plainly that it cannot measure anything
// rather than inventing zeros a person might believe.
var errUnsupported = errors.New("disk-space reads /proc and statfs: build it for linux/amd64")

func statfsDefault(string) (statfsFields, error) {
	return statfsFields{}, errUnsupported
}

func readFileDefault(string) ([]byte, error) {
	return nil, errUnsupported
}

// sourceName is "unsupported" rather than a friendly lie: a card whose footer
// said "procfs" on a build that never read /proc would be worse than one that
// admits it has nothing.
func sourceName() string { return "unsupported" }
