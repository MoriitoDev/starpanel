package themes

import (
	"os"
	"path/filepath"
	"testing"
)

const named = "/* @name: Dracula Nights */\n:root { --accent: #ff79c6; }\n"
const anonymous = ":root { --accent: #00ff00; }\n"

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store, dir
}

func TestImportTakesTheNameFromTheHeader(t *testing.T) {
	store, dir := newStore(t)

	theme, err := store.Add([]byte(named))
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if theme.Name != "Dracula Nights" {
		t.Errorf("name = %q, want the header's", theme.Name)
	}
	if theme.Slug != "dracula-nights" {
		t.Errorf("slug = %q, want dracula-nights", theme.Slug)
	}
	if _, err := os.Stat(filepath.Join(dir, "dracula-nights.css")); err != nil {
		t.Errorf("the file is not where the name says: %v", err)
	}
}

// A file someone wrote in a hurry still has to be usable and, above all,
// has to keep the same identity the next time the folder is read.
func TestImportNamesAnUnlabelledFile(t *testing.T) {
	store, _ := newStore(t)

	first, err := store.Add([]byte(anonymous))
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	second, err := store.Add([]byte(anonymous))
	if err != nil {
		t.Fatalf("add again: %v", err)
	}
	if first.Slug != "theme-1" || second.Slug != "theme-2" {
		t.Errorf("slugs = %q, %q, want theme-1 and theme-2", first.Slug, second.Slug)
	}
	if got := store.List(); len(got) != 2 {
		t.Fatalf("list = %+v, want two", got)
	}
}

func TestImportRefusesANameThatIsTaken(t *testing.T) {
	store, _ := newStore(t)
	if _, err := store.Add([]byte(named)); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, err := store.Add([]byte(named))
	if err == nil {
		t.Fatal("a second theme with the same name was accepted")
	}
	if !os.IsExist(err) && err.Error() == "" {
		t.Errorf("error = %v, want something the UI can explain", err)
	}
	if got := store.List(); len(got) != 1 {
		t.Errorf("list = %+v, want the first theme only", got)
	}
}

// A file dropped in by hand is a first-class theme: the folder is the source
// of truth, not an import log.
func TestListPicksUpAHandDroppedFile(t *testing.T) {
	store, dir := newStore(t)
	if err := os.WriteFile(filepath.Join(dir, "homemade.css"), []byte(named), 0o644); err != nil {
		t.Fatalf("drop file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	list := store.List()
	if len(list) != 1 {
		t.Fatalf("list = %+v, want the one stylesheet", list)
	}
	if list[0].Slug != "homemade" {
		t.Errorf("slug = %q, want the file's own name", list[0].Slug)
	}
}

func TestReadAndDelete(t *testing.T) {
	store, _ := newStore(t)
	theme, err := store.Add([]byte(named))
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	css, ok := store.Read(theme.Slug)
	if !ok {
		t.Fatal("Read missed a theme that exists")
	}
	if string(css) != named {
		t.Errorf("read %q, want the file back unchanged", string(css))
	}
	if _, ok := store.Read("nope"); ok {
		t.Error("Read invented a theme")
	}
	if _, ok := store.Find("nope"); ok {
		t.Error("Find invented a theme")
	}

	if err := store.Delete(theme.Slug); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(store.List()) != 0 {
		t.Errorf("list = %+v, want empty after the delete", store.List())
	}
	if err := store.Delete(theme.Slug); err == nil {
		t.Error("deleting a theme twice was accepted")
	}
}

// The slug is what goes in a URL and in the document, so it may not walk out
// of the folder.
func TestSlugsCannotEscapeTheFolder(t *testing.T) {
	store, dir := newStore(t)
	for _, name := range []string{"../evil", "a/b", "..", "", "  "} {
		if _, err := store.Add([]byte("/* @name: " + name + " */\n:root{}")); err != nil {
			continue // refused outright, which is the other good answer
		}
		for _, theme := range store.List() {
			if filepath.Dir(theme.Slug) != "." || theme.Slug == "" {
				t.Errorf("importing %q produced the slug %q", name, theme.Slug)
			}
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil.css")); err == nil {
			t.Fatalf("importing %q wrote outside the themes folder", name)
		}
	}
}
