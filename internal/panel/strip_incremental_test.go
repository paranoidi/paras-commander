package panel

import (
	"fmt"
	"maps"
	"math/rand"
	"reflect"
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/selstrip"
)

// stripTestState is a panel in /harbor/cwd whose strip is forced on, with n selections spread
// over nested directories outside the cwd, its strip rows built once.
func stripTestState(n int, tree bool) *State {
	s := &State{Path: pathloc.MustParse("/harbor/cwd"), StripForced: true}
	s.Strip.Tree = tree
	for i := range n {
		s.applySelectionAdd(stripTestPath(i), false)
	}
	_ = s.SelectedStripRows()
	return s
}

func stripTestPath(i int) string {
	return fmt.Sprintf("/harbor/meadow%03d/lantern%02d/willow_%06d.txt", i%400, i%17, i)
}

// The strip rows moved one selection at a time equal the rows built fresh from the
// selection, in both modes, with the tree kept in step instead of rebuilt.
func TestSelectedStripIncrementalMatchesFresh(t *testing.T) {
	t.Parallel()
	words := []string{"/harbor/cwd/ember.txt", "/harbor/cwd/sub/delta.txt", "/harbor/crystal/bravo.txt",
		"/harbor/crystal/bravo/raven.txt", "/harbor/crystal.txt", "/harbor/ocean/tide/foam.txt",
		"/harbor/ocean/tide/spray.txt", "/harbor/ocean/reef.txt", "/quarry/slate.txt"}
	for _, tree := range []bool{false, true} {
		rng := rand.New(rand.NewSource(7))
		s := &State{Path: pathloc.MustParse("/harbor/cwd"), StripForced: true}
		s.Strip.Tree = tree
		for step := range 300 {
			p := words[rng.Intn(len(words))]
			if s.SelectedPaths[p] {
				s.applySelectionRemove(p, false)
			} else {
				s.applySelectionAdd(p, false)
			}
			want := slices.Sorted(maps.Keys(s.SelectedPaths))
			if got := s.SelectedStripPaths(); !slices.Equal(got, want) && len(want) > 0 {
				t.Fatalf("tree=%v step %d: paths %v, want %v", tree, step, got, want)
			}
			if got, fresh := s.SelectedStripRows(), selstrip.Rows(want, "/harbor/cwd", tree, nil); len(want) > 0 && !reflect.DeepEqual(got, fresh) {
				t.Fatalf("tree=%v step %d: rows\n%+v\nwant\n%+v", tree, step, got, fresh)
			}
		}
	}
}

// A rebuild lists the selection sorted, the order a single toggle keeps.
func TestSelectedStripPathsSorted(t *testing.T) {
	t.Parallel()
	s := &State{Path: pathloc.MustParse("/harbor/cwd")}
	for _, p := range []string{"/harbor/zephyr.txt", "/harbor/amber.txt", "/harbor/mango.txt"} {
		s.applySelectionAdd(p, false)
	}
	s.invalidateSelectionDerived()
	if got, want := s.SelectedStripPaths(), []string{"/harbor/amber.txt", "/harbor/mango.txt", "/harbor/zephyr.txt"}; !slices.Equal(got, want) {
		t.Fatalf("paths %v, want %v", got, want)
	}
}

func benchStripToggle(b *testing.B, n int, tree bool) {
	s := stripTestState(n, tree)
	p := "/harbor/meadow123/lantern05/zz_extra.txt"
	b.ReportAllocs()
	for b.Loop() {
		s.applySelectionAdd(p, false)
		_ = s.SelectedStripRows()
		s.applySelectionRemove(p, false)
		_ = s.SelectedStripRows()
	}
}

// BenchmarkSelectedStripToggle* select and unselect one file with 10k selections listed in the
// strip, reading the rows after each (the strip redraw).
func BenchmarkSelectedStripToggleFlat10k(b *testing.B) { benchStripToggle(b, 10000, false) }
func BenchmarkSelectedStripToggleTree10k(b *testing.B) { benchStripToggle(b, 10000, true) }

// RemoveSelections unselects in bulk: it counts only selected paths, keeps the strip and dir
// marks consistent with the remaining selection and clears the state once nothing is left.
func TestRemoveSelections(t *testing.T) {
	t.Parallel()
	s := &State{Path: pathloc.MustParse("/harbor/cwd")}
	for _, p := range []string{"/harbor/amber.txt", "/harbor/mango.txt", "/harbor/zephyr.txt"} {
		s.applySelectionAdd(p, false)
	}
	s.applySelectionAdd("/harbor/orchard", true)
	_ = s.SelectedStripRows()
	if n := s.RemoveSelections([]string{"/harbor/mango.txt", "/harbor/orchard", "/harbor/never.txt", ""}); n != 2 {
		t.Fatalf("removed %d, want 2", n)
	}
	if got, want := s.SelectedStripPaths(), []string{"/harbor/amber.txt", "/harbor/zephyr.txt"}; !slices.Equal(got, want) {
		t.Fatalf("strip paths %v, want %v", got, want)
	}
	if s.SelectionHasDirs() {
		t.Fatalf("dir marks left after removing the only directory: %v", s.SelectedDirPaths)
	}
	if n := s.RemoveSelections([]string{"/harbor/amber.txt", "/harbor/zephyr.txt"}); n != 2 || s.SelectedPaths != nil {
		t.Fatalf("removed %d, selection %v; want 2 and cleared", n, s.SelectedPaths)
	}
}

// BenchmarkUnselectHalfBulk unselects 5k of 10k strip-listed selections in one RemoveSelections
// call and reads the strip (one by one, the same work took about 18 s).
func BenchmarkUnselectHalfBulk(b *testing.B) {
	paths := make([]string, 0, 5000)
	for i := 0; i < 10000; i += 2 {
		paths = append(paths, stripTestPath(i))
	}
	for b.Loop() {
		b.StopTimer()
		s := stripTestState(10000, true)
		b.StartTimer()
		s.RemoveSelections(paths)
		_ = s.SelectedStripRows()
	}
}

// The subtree counts moved one select/unselect at a time (nested selections, selected
// directories with selections below them, the strip shown or hidden) equal a fresh count after
// every step, and HasSelectionInSubtree follows them.
func TestSubtreeCountsIncrementalMatchRebuild(t *testing.T) {
	t.Parallel()
	pool := []string{"/harbor/cwd/ember.txt", "/harbor/cwd/sub", "/harbor/cwd/sub/delta.txt",
		"/harbor/crystal", "/harbor/crystal/bravo.txt", "/harbor/crystal/bravo/raven.txt",
		"/harbor/ocean/tide/foam.txt", "/harbor/ocean/reef.txt", "/quarry/slate.txt"}
	dirs := []string{"/", "/harbor", "/harbor/cwd", "/harbor/cwd/sub", "/harbor/crystal",
		"/harbor/crystal/bravo", "/harbor/ocean", "/harbor/ocean/tide", "/quarry", "/meadow"}
	rng := rand.New(rand.NewSource(11))
	s := &State{Path: pathloc.MustParse("/harbor/cwd")}
	for step := range 400 {
		p := pool[rng.Intn(len(pool))]
		if s.SelectedPaths[p] {
			s.applySelectionRemove(p, false)
		} else {
			s.applySelectionAdd(p, false)
		}
		if rng.Intn(5) == 0 {
			s.applySelectionAdd(p, false) // a repeated add must not count twice
		}
		_ = s.HasSelectionInSubtree("/harbor") // keep the cache built so the next step patches it
		want := buildSubtreeSelAncestors(s.SelectedPaths)
		if got := s.selDerivedCache.subtreeAncestors; len(s.SelectedPaths) > 0 && !maps.Equal(got, want) {
			t.Fatalf("step %d: counts %v, want %v", step, got, want)
		}
		for _, d := range dirs {
			if got, want := s.HasSelectionInSubtree(d), want[d] > 0; got != want {
				t.Fatalf("step %d: HasSelectionInSubtree(%s) = %v, want %v", step, d, got, want)
			}
		}
	}
}
