package pathpick

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitPathAtCursor(t *testing.T) {
	t.Parallel()
	dir, partial := splitPathAtCursor("/home/par", 9)
	if dir != "/home/" || partial != "par" {
		t.Fatalf("got dir=%q partial=%q", dir, partial)
	}
	dir, partial = splitPathAtCursor("sub/file", 6)
	if dir != "sub/" || partial != "fi" {
		t.Fatalf("rel: dir=%q partial=%q", dir, partial)
	}
}

func TestSuggestAtCursor(t *testing.T) {
	root := t.TempDir()
	fooDir := filepath.Join(root, "foo")
	barDir := filepath.Join(root, "bar")
	proj1 := filepath.Join(root, "project1")
	proj2 := filepath.Join(root, "project2")
	for _, p := range []string{fooDir, barDir, proj1, proj2} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	panel := root
	home := "/home/user"

	t.Run("non_pathlike", func(t *testing.T) {
		_, ok := SuggestAtCursor(panel, home, "fuzzy", 1, true)
		if ok {
			t.Fatal("expected no suggestion for fuzzy filter")
		}
	})

	t.Run("single_dir_match", func(t *testing.T) {
		raw := filepath.Join(root, "f")
		c, ok := SuggestAtCursor(panel, home, raw, len([]rune(raw)), true)
		if !ok || c.Suffix != "oo" || !c.IsDir {
			t.Fatalf("got %+v ok=%v want suffix oo isDir", c, ok)
		}
	})

	t.Run("lcp_multiple", func(t *testing.T) {
		raw := filepath.Join(root, "proj")
		c, ok := SuggestAtCursor(panel, home, raw, len([]rune(raw)), true)
		if !ok || c.Suffix != "ect" || c.IsDir {
			t.Fatalf("got %+v ok=%v want suffix ect isDir false", c, ok)
		}
	})

	t.Run("missing_dir", func(t *testing.T) {
		raw := filepath.Join(root, "nope", "x")
		_, ok := SuggestAtCursor(panel, home, raw, len([]rune(raw)), true)
		if ok {
			t.Fatal("expected no suggestion for missing parent")
		}
	})

	t.Run("hide_dotfiles", func(t *testing.T) {
		hidden := filepath.Join(root, ".secret")
		if err := os.Mkdir(hidden, 0o700); err != nil {
			t.Fatal(err)
		}
		raw := filepath.Join(root, ".s")
		_, ok := SuggestAtCursor(panel, home, raw, len([]rune(raw)), false)
		if ok {
			t.Fatal("expected hidden entry skipped when showHidden false")
		}
		c, ok := SuggestAtCursor(panel, home, raw, len([]rune(raw)), true)
		if !ok || c.Suffix != "ecret" {
			t.Fatalf("showHidden: got %+v ok=%v", c, ok)
		}
	})

	t.Run("relative", func(t *testing.T) {
		raw := "./f"
		c, ok := SuggestAtCursor(panel, home, raw, len([]rune(raw)), true)
		if !ok || c.Suffix != "oo" {
			t.Fatalf("relative: got %+v ok=%v", c, ok)
		}
	})

	t.Run("mid_line_cursor", func(t *testing.T) {
		proj := filepath.Join(root, "paras-commander")
		if err := os.Mkdir(proj, 0o755); err != nil {
			t.Fatal(err)
		}
		raw := proj
		runes := []rune(raw)
		mid := len(runes) - len("commander")
		_, ok := SuggestAtCursor(panel, home, raw, mid, true)
		if ok {
			t.Fatal("expected no suggestion when cursor is not at end of line")
		}
		prefix := string(runes[:mid])
		c, ok := SuggestAtCursor(panel, home, prefix, mid, true)
		if !ok || c.Suffix != "commander" {
			t.Fatalf("end of partial segment: got %+v ok=%v", c, ok)
		}
	})
}
