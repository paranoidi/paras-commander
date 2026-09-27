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

func candidateNames(items []Candidate) []string {
	names := make([]string, len(items))
	for i, c := range items {
		names[i] = c.Name
	}
	return names
}

func TestSuggestRanking(t *testing.T) {
	root := t.TempDir()
	names := []string{"paras-commander", "parsnip-garden", "copper-kettle"}
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	home := "/home/user"

	suggest := func(partial string) []Candidate {
		raw := filepath.Join(root, partial)
		_, items, ok := Suggest(root, home, raw, len([]rune(raw)), true, false)
		if !ok {
			t.Fatalf("partial %q: expected ok", partial)
		}
		return items
	}

	t.Run("p_prefix_both_first", func(t *testing.T) {
		got := candidateNames(suggest("p"))
		if len(got) < 2 || got[0] != "paras-commander" || got[1] != "parsnip-garden" {
			t.Fatalf("got %v, want [paras-commander parsnip-garden ...]", got)
		}
	})

	t.Run("pa_only_prefix_matches", func(t *testing.T) {
		got := candidateNames(suggest("pa"))
		if len(got) != 2 || got[0] != "paras-commander" || got[1] != "parsnip-garden" {
			t.Fatalf("got %v, want exactly [paras-commander parsnip-garden]", got)
		}
	})

	t.Run("comman_fuzzy_only_paras", func(t *testing.T) {
		got := candidateNames(suggest("comman"))
		if len(got) != 1 || got[0] != "paras-commander" {
			t.Fatalf("got %v, want [paras-commander]", got)
		}
	})

	t.Run("co_prefix_then_fuzzy", func(t *testing.T) {
		got := candidateNames(suggest("co"))
		if len(got) != 2 || got[0] != "copper-kettle" || got[1] != "paras-commander" {
			t.Fatalf("got %v, want [copper-kettle paras-commander]", got)
		}
	})
}

func TestSuggest(t *testing.T) {
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
		_, _, ok := Suggest(panel, home, "fuzzy", 1, true, false)
		if ok {
			t.Fatal("expected no suggestion for fuzzy filter")
		}
	})

	t.Run("single_dir_match", func(t *testing.T) {
		raw := filepath.Join(root, "f")
		_, items, ok := Suggest(panel, home, raw, len([]rune(raw)), true, false)
		if !ok || len(items) != 1 || items[0].Name != "foo" || !items[0].IsDir {
			t.Fatalf("got %+v ok=%v", items, ok)
		}
	})

	t.Run("multiple_matches_no_common_extension", func(t *testing.T) {
		raw := filepath.Join(root, "proj")
		_, items, ok := Suggest(panel, home, raw, len([]rune(raw)), true, false)
		if !ok || len(items) != 2 {
			t.Fatalf("got %+v ok=%v, want 2 items", items, ok)
		}
		names := candidateNames(items)
		if names[0] != "project1" || names[1] != "project2" {
			t.Fatalf("got %v, want sorted [project1 project2]", names)
		}
	})

	t.Run("missing_dir", func(t *testing.T) {
		raw := filepath.Join(root, "nope", "x")
		_, _, ok := Suggest(panel, home, raw, len([]rune(raw)), true, false)
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
		_, _, ok := Suggest(panel, home, raw, len([]rune(raw)), false, false)
		if ok {
			t.Fatal("expected hidden entry skipped when showHidden false")
		}
		_, items, ok := Suggest(panel, home, raw, len([]rune(raw)), true, false)
		if !ok || len(items) != 1 || items[0].Name != ".secret" {
			t.Fatalf("showHidden: got %+v ok=%v", items, ok)
		}
	})

	t.Run("relative", func(t *testing.T) {
		raw := "./f"
		_, items, ok := Suggest(panel, home, raw, len([]rune(raw)), true, false)
		if !ok || len(items) != 1 || items[0].Name != "foo" {
			t.Fatalf("relative: got %+v ok=%v", items, ok)
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
		_, _, ok := Suggest(panel, home, raw, mid, true, false)
		if ok {
			t.Fatal("expected no suggestion when cursor is not at end of line")
		}
	})

	t.Run("empty_partial_lists_all_sorted", func(t *testing.T) {
		raw := root + string(filepath.Separator)
		_, items, ok := Suggest(panel, home, raw, len([]rune(raw)), false, false)
		if !ok {
			t.Fatal("expected suggestions for trailing separator")
		}
		names := candidateNames(items)
		for i := 1; i < len(names); i++ {
			if names[i-1] > names[i] {
				t.Fatalf("not sorted: %v", names)
			}
		}
	})

	t.Run("exact_single_non_dir_match_has_nothing_to_complete", func(t *testing.T) {
		raw := filepath.Join(root, "readme.txt")
		_, _, ok := Suggest(panel, home, raw, len([]rune(raw)), true, false)
		if ok {
			t.Fatal("expected no suggestion when the only candidate exactly equals the typed name")
		}
	})
}

func TestSuggestDirsOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "lantern-meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lantern-notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(root, "lan")
	_, items, ok := Suggest(root, "/home/user", raw, len([]rune(raw)), true, true)
	if got := candidateNames(items); !ok || len(got) != 1 || got[0] != "lantern-meadow" {
		t.Fatalf("got %v ok=%v, want [lantern-meadow]", got, ok)
	}
}
