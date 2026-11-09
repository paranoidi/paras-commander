package fileindex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/paranoidi/fileindex/db"
)

// uniqueTree indexes a tree of random-word files and returns its root and the index.
func uniqueTree(t *testing.T, files map[string][]byte) (string, *Index) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeUniqueFiles(t, root, files)
	x := testIndex(t, initIndex(t))
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	return root, x
}

func writeUniqueFiles(t *testing.T, root string, files map[string][]byte) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// uniqueRels is the listed files relative to root, sorted.
func uniqueRels(t *testing.T, root string, files []db.Found) []string {
	t.Helper()
	out := make([]string, 0, len(files))
	for _, f := range files {
		rel, err := filepath.Rel(root, f.Path)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rel)
	}
	return out
}

func TestUniqueFiles(t *testing.T) {
	root, x := uniqueTree(t, map[string][]byte{
		"harbor/lone.txt":        []byte("only one of me"),
		"harbor/meadow.txt":      []byte("clover and thistle"),
		"orchard/clover.txt":     []byte("clover and thistle"),
		"harbor/twin-east.txt":   []byte("two inside"),
		"harbor/deep/twin-w.txt": []byte("two inside"),
		"harbor/lantern.jpg":     jpegWithExif(t, "camera A"),
		"orchard/pebble.jpg":     jpegWithExif(t, "camera B, longer"),
		"harbor/tundra.txt":      nil,
		"harbor/spruce.txt":      []byte("sibling prefix"),
		"harbor-north/fern.txt":  []byte("sibling prefix"),
		"orchard/bramble.txt":    []byte("a lone patch"),
	})
	at := func(dir string) string { return filepath.Join(root, dir) }

	for _, tc := range []struct {
		name  string
		scope []string
		want  []string
	}{
		{
			// meadow, lantern (metadata only) and spruce (a directory sharing the name's prefix is
			// outside) have copies outside; both twins stay; the empty file is skipped.
			"one directory", []string{at("harbor")},
			[]string{"harbor/deep/twin-w.txt", "harbor/lone.txt", "harbor/twin-east.txt"},
		},
		{
			// A copy in the other selected directory is not outside the scope.
			"two directories", []string{at("harbor"), at("orchard")},
			[]string{
				"harbor/deep/twin-w.txt", "harbor/lantern.jpg", "harbor/lone.txt", "harbor/meadow.txt",
				"harbor/twin-east.txt", "orchard/bramble.txt", "orchard/clover.txt", "orchard/pebble.jpg",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := x.UniqueFiles(tc.scope)
			if err != nil {
				t.Fatal(err)
			}
			if got := uniqueRels(t, root, res.Files); !slices.Equal(got, tc.want) {
				t.Fatalf("files = %v, want %v", got, tc.want)
			}
			if len(res.Unverified) != 0 || len(res.NotIndexed) != 0 {
				t.Fatalf("unverified = %v, notIndexed = %v", res.Unverified, res.NotIndexed)
			}
		})
	}
}

func TestUniqueFilesFlagsWhatItCannotRuleOut(t *testing.T) {
	head := func(first byte) []byte {
		b := make([]byte, 9000)
		b[0] = first
		return b
	}
	root, x := uniqueTree(t, map[string][]byte{
		"orchard/anchor.bin": head('a'),
		"harbor/lone.txt":    []byte("only one of me"),
	})
	writeUniqueFiles(t, root, map[string][]byte{
		"harbor/willow.bin":  head('a'), // same size and head as a file outside
		"harbor/sparrow.bin": head('c'), // no match, but not fully hashed
	})
	if err := x.ScanFastTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	res, err := x.UniqueFiles([]string{filepath.Join(root, "harbor")})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"harbor/lone.txt", "harbor/sparrow.bin", "harbor/willow.bin"}
	if got := uniqueRels(t, root, res.Files); !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	for rel, flagged := range map[string]bool{"harbor/lone.txt": false, "harbor/sparrow.bin": true, "harbor/willow.bin": true} {
		if res.Unverified[filepath.Join(root, rel)] != flagged {
			t.Errorf("%s: unverified = %v, want %v", rel, !flagged, flagged)
		}
	}
	// The other side of the potential match is fully hashed, and still cannot be ruled out.
	res, err = x.UniqueFiles([]string{filepath.Join(root, "orchard")})
	if err != nil {
		t.Fatal(err)
	}
	if got := uniqueRels(t, root, res.Files); !slices.Equal(got, []string{"orchard/anchor.bin"}) || !res.Unverified[res.Files[0].Path] {
		t.Fatalf("files = %v, unverified = %v", got, res.Unverified)
	}
}

func TestUniqueFilesReportsDirectoriesNotIndexed(t *testing.T) {
	root, x := uniqueTree(t, map[string][]byte{"harbor/lone.txt": []byte("only one of me")})
	missing := filepath.Join(root, "thistle")
	if err := os.Mkdir(missing, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := x.UniqueFiles([]string{missing})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 0 || !slices.Equal(res.NotIndexed, []string{missing}) {
		t.Fatalf("nothing indexed: files = %v, notIndexed = %v", res.Files, res.NotIndexed)
	}
	res, err = x.UniqueFiles([]string{filepath.Join(root, "harbor"), missing})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || !slices.Equal(res.NotIndexed, []string{missing}) {
		t.Fatalf("partly indexed: files = %v, notIndexed = %v", res.Files, res.NotIndexed)
	}
}

func TestFilterUnique(t *testing.T) {
	const root = "/srv/harbor"
	var files []db.Found
	for _, p := range []string{"Lantern.txt", "meadow.jpg", "deep/lantern-two.txt", ".hidden/willow.txt", "fern[1].txt"} {
		path := filepath.Join(root, p)
		files = append(files, db.Found{Name: filepath.Base(path), Path: path})
	}
	original := slices.Clone(files)
	paths := func(found []db.Found) []string {
		return uniqueRels(t, root, found)
	}
	for _, tc := range []struct {
		name      string
		q         FindQuery
		limit     int
		want      []string
		truncated bool
	}{
		{name: "blank lists all", q: FindQuery{Text: "  ", IncludeHidden: true}, want: []string{"Lantern.txt", "meadow.jpg", "deep/lantern-two.txt", ".hidden/willow.txt", "fern[1].txt"}},
		{name: "substring ignores case", q: FindQuery{Text: "LANTERN", IncludeHidden: true}, want: []string{"Lantern.txt", "deep/lantern-two.txt"}},
		{name: "glob matches the whole name", q: FindQuery{Text: "*.jpg", IncludeHidden: true}, want: []string{"meadow.jpg"}},
		{name: "question mark", q: FindQuery{Text: "?eadow.jpg", IncludeHidden: true}, want: []string{"meadow.jpg"}},
		{name: "bracket is literal", q: FindQuery{Text: "[1]", IncludeHidden: true}, want: []string{"fern[1].txt"}},
		{name: "hidden off", q: FindQuery{Text: "*"}, want: []string{"Lantern.txt", "meadow.jpg", "deep/lantern-two.txt", "fern[1].txt"}},
		{name: "depth", q: FindQuery{Text: "lantern", MaxDepth: 1, IncludeHidden: true}, want: []string{"Lantern.txt"}},
		{name: "cap", q: FindQuery{IncludeHidden: true}, limit: 2, want: []string{"Lantern.txt", "meadow.jpg"}, truncated: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, truncated := FilterUnique(files, root, tc.q, tc.limit)
			if !slices.Equal(paths(got), tc.want) || truncated != tc.truncated {
				t.Fatalf("got %v truncated=%v, want %v truncated=%v", paths(got), truncated, tc.want, tc.truncated)
			}
			if !slices.Equal(files, original) {
				t.Fatalf("the loaded list changed: %v", files)
			}
		})
	}
}
