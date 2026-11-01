package scan

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// writeCriteriaTree builds a small tree: an old and a fresh file, an empty file, an empty and a
// full directory, an excluded directory and a broken symlink.
func writeCriteriaTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "orchard", "meadow"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "hollow"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "cellar", "attic"), 0o755))
	must(os.WriteFile(filepath.Join(root, "orchard", "lantern.go"), make([]byte, 4096), 0o644))
	must(os.WriteFile(filepath.Join(root, "orchard", "meadow", "pebble.txt"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "harbor.md"), nil, 0o644))
	must(os.WriteFile(filepath.Join(root, "cellar", "attic", "ember.go"), []byte("x"), 0o644))
	must(os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "willow")))
	old := time.Now().Add(-30 * 24 * time.Hour)
	must(os.Chtimes(filepath.Join(root, "orchard", "meadow", "pebble.txt"), old, old))
	return root
}

func walkAll(t *testing.T, root string, opts WalkOptions) map[string]Entry {
	t.Helper()
	opts.Root = root
	w := startRootWalk(t.Context(), root, opts)
	out := map[string]Entry{}
	for batch := range w.Results() {
		for _, e := range batch {
			out[e.RelLine] = e
		}
	}
	<-w.Done()
	return out
}

func TestWalkStatCollectsMeta(t *testing.T) {
	t.Parallel()
	root := writeCriteriaTree(t)

	plain := walkAll(t, root, WalkOptions{IncludeHidden: true})
	if e := plain["orchard/lantern.go"]; e.Meta != nil || e.Size != 0 {
		t.Fatalf("unstatted walk: Meta=%v Size=%d, want none", e.Meta, e.Size)
	}
	if !plain["willow"].Broken {
		t.Fatal("broken symlink not flagged without stat")
	}

	got := walkAll(t, root, WalkOptions{IncludeHidden: true, Stat: findcrit.StatMetaEmptyDirs})
	lantern := got["orchard/lantern.go"]
	if lantern.Meta == nil || lantern.Size != 4096 {
		t.Fatalf("lantern: Meta=%v Size=%d, want metadata and 4096", lantern.Meta, lantern.Size)
	}
	if lantern.Meta.UID != uint32(os.Getuid()) {
		t.Errorf("lantern UID = %d, want %d", lantern.Meta.UID, os.Getuid())
	}
	pebble := got["orchard/meadow/pebble.txt"]
	if pebble.Meta == nil || time.Since(time.Unix(0, pebble.Meta.ModTime)) < 29*24*time.Hour {
		t.Fatalf("pebble mtime not collected: %+v", pebble.Meta)
	}
	if m := got["hollow"].Meta; m == nil || !m.EmptyDir {
		t.Errorf("hollow: want EmptyDir, got %+v", m)
	}
	if m := got["orchard"].Meta; m == nil || m.EmptyDir {
		t.Errorf("orchard: want non-empty, got %+v", m)
	}
}

func TestWalkExcludePrunes(t *testing.T) {
	t.Parallel()
	root := writeCriteriaTree(t)
	ex, err := findcrit.ParseExcludes("cellar, *.md")
	if err != nil {
		t.Fatal(err)
	}
	got := walkAll(t, root, WalkOptions{
		IncludeHidden: true,
		Exclude:       func(abs string) bool { return ex.Match(relLine(root, abs)) },
	})
	for rel := range got {
		if rel == "cellar" || rel == "cellar/attic" || rel == "cellar/attic/ember.go" || rel == "harbor.md" {
			t.Errorf("excluded entry indexed: %q", rel)
		}
	}
	if _, ok := got["orchard/lantern.go"]; !ok {
		t.Error("non-excluded entry missing")
	}
}

func matchedRels(entries []Entry, out MatchOutput) []string {
	rels := make([]string, 0, len(out.FullRanked))
	for _, i := range out.FullRanked {
		rels = append(rels, entries[i].RelLine)
	}
	return rels
}

func TestRunMatchCriteriaAndModes(t *testing.T) {
	t.Parallel()
	root := writeCriteriaTree(t)
	walked := walkAll(t, root, WalkOptions{IncludeHidden: true, Stat: findcrit.StatMetaEmptyDirs})
	entries := make([]Entry, 0, len(walked))
	for _, e := range walked {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].RelLine < entries[j].RelLine })

	week := time.Now().Add(-7 * 24 * time.Hour).UnixNano()
	tests := []struct {
		name string
		req  MatchRequest
		want []string
	}{
		{"empty query + empty criterion", MatchRequest{Criteria: findcrit.Criteria{Empty: true}}, []string{"harbor.md", "hollow"}},
		{"older than a week", MatchRequest{Criteria: findcrit.Criteria{Modified: findcrit.TimeBound{Older: week}}}, []string{"orchard/meadow/pebble.txt"}},
		{"min size", MatchRequest{Criteria: findcrit.Criteria{MinSize: 1024}}, []string{"orchard/lantern.go"}},
		{"broken symlinks", MatchRequest{Criteria: findcrit.Criteria{BrokenSymlinks: true}}, []string{"willow"}},
		{"glob in walk order", MatchRequest{Query: "*.go", Mode: findcrit.ModeGlob, CaseInsensitive: true}, []string{"cellar/attic/ember.go", "orchard/lantern.go"}},
		{"regex", MatchRequest{Query: `^orchard/.*\.(txt|go)$`, Mode: findcrit.ModeRegex}, []string{"orchard/lantern.go", "orchard/meadow/pebble.txt"}},
		{"glob + criteria", MatchRequest{Query: "*.go", Mode: findcrit.ModeGlob, Criteria: findcrit.Criteria{MinSize: 1024}}, []string{"orchard/lantern.go"}},
		{"invalid regex matches nothing", MatchRequest{Query: "(", Mode: findcrit.ModeRegex}, nil},
	}
	for _, tt := range tests {
		out := runMatchInPlace(entries, tt.req, nil)
		got := matchedRels(entries, out)
		if len(got) != len(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}

func TestEnrichHiddenStatsAndExcludes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".lantern"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, ".willow")); err != nil {
		t.Fatal(err)
	}
	s := newHiddenWorkTestSession(root)
	s.opts.Stat = findcrit.StatMeta
	s.opts.Exclude = findcrit.Excludes{".ember"}
	batch := s.enrichHidden([]Entry{{RelLine: ".lantern"}, {RelLine: ".willow"}, {RelLine: ".ember"}})
	if len(batch) != 2 {
		t.Fatalf("enrichHidden kept %d entries, want 2 (exclude drops .ember)", len(batch))
	}
	if batch[0].Meta == nil || batch[0].Size != 3 {
		t.Errorf(".lantern: Meta=%v Size=%d, want metadata and 3", batch[0].Meta, batch[0].Size)
	}
	if !batch[1].Broken || batch[1].Type != localfs.EntrySymlink {
		t.Errorf(".willow: want a broken symlink, got %+v", batch[1])
	}
}
