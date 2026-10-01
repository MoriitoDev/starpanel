package plugins

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrExists reports an import whose Plugin name is already taken. The panel
// refuses rather than overwriting: a Plugin folder is somebody's work, and its
// backend is a program core would run.
var ErrExists = errors.New("a plugin with that name is already installed")

// maxUnpackedBytes caps what one archive may write to disk. The transport caps
// the upload; this is the other half of the same thought, because a small
// archive can unpack into a folder far larger than itself.
const maxUnpackedBytes int64 = 64 << 20

// Import unpacks a Plugin archive into the plugins folder, and answers with
// the Entry a dropped-in folder would have produced.
//
// The Manifest inside the archive decides the name, and nothing touches the
// folder until it has been read: the archive is unpacked into a staging folder
// that is thrown away on every failure, so a broken upload leaves nothing
// behind — not a half-written Plugin, and not the archive, which is never a
// file on disk at all.
func (r *Registry) Import(archive []byte) (Entry, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return Entry{}, fmt.Errorf("read the archive: %w", err)
	}
	root, declared, err := manifestIn(reader)
	if err != nil {
		return Entry{}, err
	}
	// The name is about to become a folder name, so the rule that governs
	// names is applied before it does.
	if err := validName(declared.Name); err != nil {
		return Entry{}, err
	}
	destination := filepath.Join(r.dir, declared.Name)
	if _, err := os.Stat(destination); err == nil {
		return refused(declared.Name, destination)
	}
	// A panel whose plugins folder has never held anything has no plugins
	// folder yet: discovery treats that as "no Plugins" and so must an import,
	// rather than failing on the staging folder it is about to create inside.
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return Entry{}, fmt.Errorf("prepare the plugins folder: %w", err)
	}

	staging, err := os.MkdirTemp(r.dir, ".import-")
	if err != nil {
		return Entry{}, fmt.Errorf("stage the import: %w", err)
	}
	defer os.RemoveAll(staging)
	folder := filepath.Join(staging, declared.Name)
	if err := unpack(reader, root, folder); err != nil {
		return Entry{}, err
	}

	// The same validator a dropped folder goes through has the last word, so
	// an archive cannot smuggle in what the folder path would reject.
	manifest, err := LoadManifest(folder)
	if err != nil {
		return Entry{}, err
	}
	if err := os.Rename(folder, destination); err != nil {
		// Somebody else got there first between the check and the move.
		if _, statErr := os.Stat(destination); statErr == nil {
			return refused(manifest.Name, destination)
		}
		return Entry{}, fmt.Errorf("move the plugin into %s: %w", r.dir, err)
	}
	return Entry{Folder: manifest.Name, Dir: destination, Manifest: manifest}, nil
}

// refused is what an import that would overwrite a Plugin answers with: the
// name it collided with, and the folder holding it, because deleting that
// folder is the owner's move.
func refused(name, destination string) (Entry, error) {
	return Entry{Folder: name, Dir: destination}, fmt.Errorf(
		"%w: %s — delete or rename that folder first", ErrExists, destination)
}

// Archive returns a Plugin's folder as a ZIP, under the folder name, which is
// the shape Import takes back in.
//
// The mode travels with each entry, because a Plugin may carry a compiled
// binary and `writer.Create` would write 0644: a Download that cannot be
// imported back into a working Plugin is worse than no download at all.
func (e Entry) Archive() ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	err := filepath.WalkDir(e.Dir, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// A folder arrives with the file that is inside it.
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(e.Dir, file)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header := &zip.FileHeader{
			Name:     e.Folder + "/" + filepath.ToSlash(relative),
			Method:   zip.Deflate,
			Modified: info.ModTime(),
		}
		header.SetMode(info.Mode())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = entry.Write(content)
		return err
	})
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("archive plugin %q: %w", e.Folder, err)
	}
	return buffer.Bytes(), nil
}

// manifestIn finds the Manifest at an archive's root and the folder it sits
// in. Zipping a folder and zipping its contents differ by exactly that prefix,
// and a person means the Plugin either way; the shallowest Manifest wins,
// because that is the one the archive is rooted at.
func manifestIn(reader *zip.Reader) (string, Manifest, error) {
	// A Manifest at the root outranks one nested a folder deeper, so the
	// archive is looked at twice rather than guessed at once.
	for _, atRoot := range []bool{true, false} {
		for _, file := range reader.File {
			name := strings.TrimSuffix(file.Name, "/")
			if file.FileInfo().IsDir() || path.Base(name) != "manifest.json" {
				continue
			}
			depth := strings.Count(name, "/")
			if depth > 1 || atRoot != (depth == 0) {
				continue
			}
			raw, err := readEntry(file)
			if err != nil {
				return "", Manifest{}, err
			}
			var declared Manifest
			if err := json.Unmarshal(raw, &declared); err != nil {
				return "", Manifest{}, fmt.Errorf("parse manifest: %w", err)
			}
			root := ""
			if dir := path.Dir(name); dir != "." {
				root = dir
			}
			return root, declared, nil
		}
	}
	return "", Manifest{}, errors.New("the archive holds no manifest.json at its root, so there is no Plugin inside")
}

// unpack writes every entry that lives inside root into folder, refusing
// anything that would land somewhere else.
func unpack(reader *zip.Reader, root, folder string) error {
	var unpacked int64
	for _, file := range reader.File {
		relative, inside, err := entryPath(file.Name, root)
		if err != nil {
			return err
		}
		if !inside {
			continue
		}
		// A symlink is how an archive reaches outside its folder without ever
		// naming "..", and a Plugin is served as plain files, so only files
		// and folders are unpacked.
		if !file.FileInfo().Mode().IsRegular() {
			continue
		}
		target, ok := insideFolder(folder, relative)
		if !ok {
			return fmt.Errorf("archive entry %q escapes the Plugin folder", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("unpack %q: %w", file.Name, err)
		}
		written, err := writeEntry(file, target, maxUnpackedBytes-unpacked)
		if err != nil {
			return err
		}
		unpacked += written
		if unpacked > maxUnpackedBytes {
			return fmt.Errorf("the archive unpacks to more than %d MiB", maxUnpackedBytes>>20)
		}
	}
	return nil
}

// entryPath is where one archive entry belongs inside the Plugin folder. The
// second result is false for entries outside the folder the Manifest was found
// in — an archive written on a Mac carries __MACOSX beside the real one — and
// an entry that would climb out is refused rather than quietly rewritten.
func entryPath(archiveName, root string) (string, bool, error) {
	name := path.Clean(strings.TrimSuffix(archiveName, "/"))
	switch {
	case name == "." || name == root:
		return "", false, nil
	case path.IsAbs(name), strings.ContainsAny(name, `\:`) || hasParentPrefix(name):
		return "", false, fmt.Errorf("archive entry %q is not a path inside a Plugin folder", archiveName)
	case root != "" && !strings.HasPrefix(name, root+"/"):
		return "", false, nil
	}
	return strings.TrimPrefix(strings.TrimPrefix(name, root), "/"), true, nil
}

// writeEntry unpacks one file, writing at most budget+1 bytes: an archive that
// lies about how big its contents are is refused rather than allowed to fill
// the disk first. The mode an archive declares is carried over, because a
// Plugin is allowed to bring its own binary — an archive may grant execute and
// nothing more, so setuid, setgid and sticky are dropped and the file is never
// world-writable.
func writeEntry(file *zip.File, target string, budget int64) (int64, error) {
	source, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("open %q in the archive: %w", file.Name, err)
	}
	defer source.Close()
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, unpackedMode(file.Mode()))
	if err != nil {
		return 0, fmt.Errorf("unpack %q: %w", file.Name, err)
	}
	written, err := io.Copy(destination, io.LimitReader(source, budget+1))
	if err != nil {
		destination.Close()
		return written, fmt.Errorf("unpack %q: %w", file.Name, err)
	}
	if err := destination.Close(); err != nil {
		return written, fmt.Errorf("unpack %q: %w", file.Name, err)
	}
	// The mode is set again after writing because the create mode is filtered
	// by the process umask, and an archive's execute bit is not the umask's
	// business. Windows has no such bit and reports a synthetic mode, so the
	// call is made only where it means something.
	if err := os.Chmod(target, unpackedMode(file.Mode())); err != nil {
		return written, fmt.Errorf("set the mode on %q: %w", file.Name, err)
	}
	return written, nil
}

// unpackedMode is what an archive entry is allowed to become. The base is
// readable and writable by its owner, which is all a Plugin's own files need;
// the only thing an archive may add to that is execute, for the binaries
// `docs/PLUGINS.md` lets a Plugin carry. Setuid, setgid, sticky and
// world-writable are dropped rather than honoured: an archive is a folder from
// a stranger, and the panel is not going to run what it hands over as anyone
// but itself.
//
// It takes a mode rather than the zip entry so the decision is a pure one, and
// a test can pin it on a platform that has no execute bit to read back.
func unpackedMode(from fs.FileMode) fs.FileMode {
	mode := fs.FileMode(0o644)
	if from.Perm()&0o111 != 0 {
		// Execute is granted on the owner, and on the group and others only if
		// the archive asked for it there. A file nobody but the owner may run
		// is exactly what a Plugin's own binary should be.
		mode |= 0o100
		if from.Perm()&0o010 != 0 {
			mode |= 0o010
		}
		if from.Perm()&0o001 != 0 {
			mode |= 0o001
		}
	}
	return mode
}

func readEntry(file *zip.File) ([]byte, error) {
	opened, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open %q in the archive: %w", file.Name, err)
	}
	defer opened.Close()
	raw, err := io.ReadAll(opened)
	if err != nil {
		return nil, fmt.Errorf("read %q in the archive: %w", file.Name, err)
	}
	return raw, nil
}
