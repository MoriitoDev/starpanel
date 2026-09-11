// Package themes owns the folder of imported Theme stylesheets.
//
// A Theme is a CSS file, so the folder is the source of truth: one dropped in
// by hand is as real as one the panel imported, and what the Dashboard stores
// is only a name. Anything the file does to the page is the author's business;
// the panel's job is to find it, name it, serve it and forget it when asked.
package themes

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ErrExists reports an import whose name is already taken. The panel refuses
// rather than overwriting: a theme is somebody's work.
var ErrExists = errors.New("a theme with that name already exists")

// ErrNotFound reports a slug with no stylesheet behind it.
var ErrNotFound = errors.New("theme not found")

// Theme is one stylesheet in the folder. Name is what its author called it —
// or the generated name it got instead — and Slug is its file name without the
// extension, which is the identity the Dashboard stores and the URL carries.
type Theme struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Store is a directory of Theme stylesheets.
type Store struct {
	dir string
}

// NewStore prepares the folder, creating it when missing, so a fresh install
// has somewhere to import into.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create themes dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// List reads the folder on every call, so a stylesheet dropped in by hand
// appears without a restart. Names come before slugs.
func (s *Store) List() []Theme {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	found := []Theme{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".css") {
			continue
		}
		slug := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		found = append(found, Theme{Name: s.nameOf(slug), Slug: slug})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Slug < found[j].Slug })
	return found
}

// Find returns the Theme behind one slug.
func (s *Store) Find(slug string) (Theme, bool) {
	if !safeSlug(slug) {
		return Theme{}, false
	}
	if _, err := os.Stat(s.path(slug)); err != nil {
		return Theme{}, false
	}
	return Theme{Name: s.nameOf(slug), Slug: slug}, true
}

// Read returns a Theme's stylesheet as it is on disk.
func (s *Store) Read(slug string) ([]byte, bool) {
	if !safeSlug(slug) {
		return nil, false
	}
	css, err := os.ReadFile(s.path(slug))
	if err != nil {
		return nil, false
	}
	return css, true
}

// Add writes a stylesheet, taking its name from the file's header comment and
// generating one when the author did not leave one. The name decides the slug,
// so what the list shows and what the folder holds always agree.
func (s *Store) Add(css []byte) (Theme, error) {
	if len(bytes.TrimSpace(css)) == 0 {
		return Theme{}, errors.New("the theme file is empty")
	}
	name := headerName(css)
	slug := slugify(name)
	if slug == "" {
		slug = s.freeSlug()
		name = slug
	}
	if _, err := os.Stat(s.path(slug)); err == nil {
		return Theme{}, fmt.Errorf("%w: %s", ErrExists, name)
	}
	if err := os.WriteFile(s.path(slug), css, 0o644); err != nil {
		return Theme{}, fmt.Errorf("write theme: %w", err)
	}
	return Theme{Name: name, Slug: slug}, nil
}

// Delete removes a Theme. It does not care whether the panel is using it: the
// Dashboard falls back to its default when the name stops resolving.
func (s *Store) Delete(slug string) error {
	if !safeSlug(slug) {
		return ErrNotFound
	}
	if err := os.Remove(s.path(slug)); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete theme: %w", err)
	}
	return nil
}

func (s *Store) path(slug string) string {
	return filepath.Join(s.dir, slug+".css")
}

// nameOf reads a theme's own name, falling back to its file name. A file the
// panel did not import has no other stable identity, and a name that changed
// between reads would be worse than a plain one.
func (s *Store) nameOf(slug string) string {
	css, err := os.ReadFile(s.path(slug))
	if err != nil {
		return slug
	}
	if name := headerName(css); name != "" {
		return name
	}
	return slug
}

// freeSlug is the next generated name, so an unlabelled import lands on a
// stable identity instead of a number that moves.
func (s *Store) freeSlug() string {
	for n := 1; ; n++ {
		slug := fmt.Sprintf("theme-%d", n)
		if _, err := os.Stat(s.path(slug)); os.IsNotExist(err) {
			return slug
		}
	}
}

// safeSlug keeps a slug from naming anything outside the folder. Slugs come
// from URLs, so this is the guard that makes lookup-by-name safe.
func safeSlug(slug string) bool {
	return slug != "" && slug == filepath.Base(slug) && !strings.ContainsAny(slug, `/\`)
}

var nameLine = regexp.MustCompile(`(?mi)^[\s*/]*@name\s*:\s*(.+?)\s*$`)

// headerName reads the name out of a leading comment block:
//
//	/* @name: Dracula Nights */
//
// Anything outside that block is stylesheet, not metadata.
func headerName(css []byte) string {
	head := bytes.TrimSpace(css)
	if !bytes.HasPrefix(head, []byte("/*")) {
		return ""
	}
	end := bytes.Index(head, []byte("*/"))
	if end < 0 {
		return ""
	}
	match := nameLine.FindSubmatch(head[:end])
	if match == nil {
		return ""
	}
	return string(bytes.TrimSpace(match[1]))
}

// slugify turns a display name into a file name: ASCII letters and digits,
// separated by single dashes.
func slugify(name string) string {
	var b strings.Builder
	dangling := false
	for _, r := range strings.ToLower(name) {
		switch fold(r) {
		case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
			'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			b.WriteRune(fold(r))
			dangling = false
		default:
			if !dangling && b.Len() > 0 {
				b.WriteByte('-')
				dangling = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// fold maps the accented letters a European theme name is likely to carry onto
// their plain forms, so "Canción" does not become "canci-n".
func fold(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'ö', 'õ':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	}
	return r
}
