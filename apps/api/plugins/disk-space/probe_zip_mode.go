//go:build ignore

// probe reads a ZIP the way core does, through archive/zip, and prints what Go
// makes of each entry's mode. It exists because the external attributes are not
// the whole story: Go only reads them as a Unix mode when the entry's "version
// made by" high byte says the archive was made on Unix. A ZIP written by
// System.IO.Compression says FAT, and then a perfectly good 0o755 in the
// external attributes is ignored — which is how a build script can look right
// and ship an unexecutable binary.
//
//	go run probe_zip_mode.go <archive.zip>
package main

import (
	"archive/zip"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: probe_zip_mode.go <archive.zip>")
		os.Exit(2)
	}
	reader, err := zip.OpenReader(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer reader.Close()
	bad := 0
	for _, file := range reader.File {
		mode := file.Mode()
		executable := mode.Perm()&0o111 != 0
		if file.Name == "disk-space/backend" || file.Name == "backend" {
			fmt.Printf("%-46s mode=%v exec=%v (creator 0x%04x)\n", file.Name, mode, executable, file.CreatorVersion)
			if !executable {
				bad = 1
			}
		}
	}
	if bad != 0 {
		fmt.Println("FAIL: the binary is not executable as Go reads it")
		os.Exit(1)
	}
	fmt.Println("OK: the binary is executable as Go reads it")
}
