package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// dedupBruteCopyRoots is the reference for a dir row: every file of the snapshot
// filtered by walking its ancestors against the twin set.
func dedupBruteCopyRoots(snap comparepkg.DedupSnapshot, twins []string) []DedupRow {
	set := map[string]bool{}
	for _, t := range twins {
		set[t] = true
	}
	roots := dedupDirRoots(snap, dedupIndexFor(snap, nil).marks, func(yield func(int, comparepkg.DedupFile) bool) {
		for gi, g := range snap.Groups {
			for _, f := range g.Files {
				for rel := f.Rel; ; {
					if set[rel] {
						yield(gi, f)
						break
					}
					i := strings.LastIndexByte(rel, '/')
					if i < 0 {
						if set[""] {
							yield(gi, f)
						}
						break
					}
					rel = rel[:i]
				}
			}
		}
	})
	return treeflat.Flatten(roots, dedupExpandedFn(nil))
}

func TestDedupCopyRowsDirIndexMatchesBruteForce(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "photo/lantern.bin", "backup/photo/lantern.bin", "photos/lantern.bin"),
			dedupTestGroup(2, 90, "photo/sub/pebble.bin", "backup/photo/sub/pebble.bin", "photos/x/pebble.bin"),
			dedupTestGroup(3, 80, "meadow/ember.bin", "other/ember.bin", "elsewhere/ember.bin"),
			dedupTestGroup(4, 70, "meadow/sub/harbor.bin", "other/sub/harbor.bin", "ember.bin"),
		},
		DirGroups: []comparepkg.DedupDirGroup{
			{Kind: comparepkg.DirExact, Size: 190, Files: 2, Rels: []string{"photo", "backup/photo"}, Hidden: []bool{false, false}},
			{Kind: comparepkg.DirContent, Size: 150, Files: 2, Rels: []string{"meadow", "meadow/sub", "other"}, Hidden: []bool{false, false, false}},
		},
	}
	for _, dirRel := range []string{"photo", "backup/photo", "meadow", "other", "meadow/sub", "photos"} {
		sel := DedupRow{Value: &DedupRowData{Kind: DedupRowDir, DirRel: dirRel}}
		cache := &DedupCache{}
		want := dedupBruteCopyRoots(snap, DedupTwinDirs(snap, dirRel, nil))
		for name, c := range map[string]*DedupCache{"nil": nil, "cache": cache, "cache again": cache} {
			got := DedupCopyRows(snap, sel, nil, c)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: DedupCopyRows(%q) = %v, want %v", name, dirRel, dedupRowIDs(got), dedupRowIDs(want))
			}
		}
	}
	sel := DedupRow{Value: &DedupRowData{Kind: DedupRowDir, DirRel: "photo"}}
	got := dedupRowIDs(DedupCopyRows(snap, sel, nil, &DedupCache{}))
	for _, id := range got {
		if strings.Contains(id, "photos") {
			t.Errorf("sibling dir leaked into twin rows: %v", got)
		}
	}
	if len(got) == 0 {
		t.Fatal("no copy rows for twin dir")
	}
	// meadow/sub is nested under twin meadow: its files appear once.
	other := DedupRow{Value: &DedupRowData{Kind: DedupRowDir, DirRel: "other"}}
	ids := dedupRowIDs(DedupCopyRows(snap, other, nil, &DedupCache{}))
	slices.Sort(ids)
	if len(slices.Compact(slices.Clone(ids))) != len(ids) {
		t.Errorf("duplicate row ids: %v", ids)
	}
}
