package panel

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"testing"

	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// setupPatternFilterState creates a directory with three files (one matching a pattern by
// basename, one only via a meta column value, one matching neither) and returns the loaded state
// plus the meta-matching file's absolute path.
func setupPatternFilterState(t *testing.T) (State, string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"thicket.txt", "lantern.txt", "sesame.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return state, filepath.Join(dir, "lantern.txt")
}

func TestPatternFilterIncludeMetaMatchesByNameOrValue(t *testing.T) {
	state, lanternPath := setupPatternFilterState(t)
	meta := func() GroupSelectMeta {
		return GroupSelectMeta{Cols: []map[string]string{{lanternPath: "beacon tag"}}}
	}
	filter, err := PatternFilter(FilterSpec{Pattern: "thicket", Mode: GroupPatternSimple}, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	if filter.ID != PatternMetaFilterID {
		t.Fatalf("ID = %q, want %q", filter.ID, PatternMetaFilterID)
	}
	state.SetEntryFilter(filter)
	// thicket.txt matches by basename; lantern.txt only via its meta value — but that value
	// doesn't contain "thicket", so only thicket.txt should be visible.
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1", got)
	}

	// Now match a pattern that only the meta value satisfies: with Include on, basename matching
	// still runs (finds nothing) and the meta value match adds lantern.txt.
	filter, err = PatternFilter(FilterSpec{Pattern: "beacon", Mode: GroupPatternSimple}, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1", got)
	}
	entry, _, ok := state.VisibleEntry(0)
	if !ok || entry.Name != "lantern.txt" {
		t.Fatalf("VisibleEntry(0) = %+v, ok=%v, want lantern.txt", entry, ok)
	}
}

func TestPatternFilterOnlyMetaSkipsBasenameMatch(t *testing.T) {
	state, lanternPath := setupPatternFilterState(t)
	meta := func() GroupSelectMeta {
		return GroupSelectMeta{Cols: []map[string]string{{lanternPath: "beacon tag"}}, OnlyMeta: true}
	}
	// "thicket" matches thicket.txt's basename, but OnlyMeta suppresses basename matching
	// entirely, so only meta-value matches count — here, none.
	filter, err := PatternFilter(FilterSpec{Pattern: "thicket", Mode: GroupPatternSimple}, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 0 {
		t.Fatalf("VisibleEntryCount = %d, want 0", got)
	}

	// "beacon" matches only lantern.txt's meta value.
	filter, err = PatternFilter(FilterSpec{Pattern: "beacon", Mode: GroupPatternSimple}, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1", got)
	}
	entry, _, ok := state.VisibleEntry(0)
	if !ok || entry.Name != "lantern.txt" {
		t.Fatalf("VisibleEntry(0) = %+v, ok=%v, want lantern.txt", entry, ok)
	}
}

// TestPatternFilterRefreshPicksUpLiveProviderData simulates meta results arriving
// asynchronously: the meta closure re-reads a variable that gets swapped (not mutated in place,
// mirroring MetaResults being replaced wholesale) after the filter is already installed, and
// RefreshEntryFilter must pick up the change without reinstalling the filter.
func TestPatternFilterRefreshPicksUpLiveProviderData(t *testing.T) {
	state, lanternPath := setupPatternFilterState(t)

	var live map[string]string // nil until the "provider" resolves
	meta := func() GroupSelectMeta {
		return GroupSelectMeta{Cols: []map[string]string{live}}
	}
	filter, err := PatternFilter(FilterSpec{Pattern: "beacon", Mode: GroupPatternSimple}, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 0 {
		t.Fatalf("VisibleEntryCount before data arrives = %d, want 0", got)
	}

	live = map[string]string{lanternPath: "beacon tag"}
	state.RefreshEntryFilter()
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount after RefreshEntryFilter = %d, want 1", got)
	}
	entry, _, ok := state.VisibleEntry(0)
	if !ok || entry.Name != "lantern.txt" {
		t.Fatalf("VisibleEntry(0) = %+v, ok=%v, want lantern.txt", entry, ok)
	}
}

func TestPatternFilterScope(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"harbor", "meadow"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"match.go", "other.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name string
		spec FilterSpec
		want []string
	}{
		{"files keeps every dir", FilterSpec{Pattern: "*.go"}, []string{"harbor", "match.go", "meadow"}},
		{"files invert keeps every dir", FilterSpec{Pattern: "*.go", Invert: true}, []string{"harbor", "meadow", "other.txt"}},
		{"dirs hides files", FilterSpec{Pattern: "harb*", Scope: ScopeDirs}, []string{"harbor"}},
		{"dirs invert hides files", FilterSpec{Pattern: "harb*", Scope: ScopeDirs, Invert: true}, []string{"meadow"}},
		{"both filters dirs like files", FilterSpec{Pattern: "*.go", Scope: ScopeBoth}, []string{"match.go"}},
		{"both matches a dir", FilterSpec{Pattern: "harb*", Scope: ScopeBoth}, []string{"harbor"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := New(dir)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			tt.spec.Mode = GroupPatternShell
			f, err := PatternFilter(tt.spec, nil)
			if err != nil {
				t.Fatalf("PatternFilter() error = %v", err)
			}
			state.SetEntryFilter(f)
			got := visibleNames(&state)
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			if hasParentEntry(&state) {
				want = append(want, "..")
			}
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("visible = %v, want %v", got, want)
			}
		})
	}
}

// hasParentEntry reports whether the unfiltered listing has a ".." row (kept by every pattern
// filter).
func hasParentEntry(s *State) bool {
	return slices.ContainsFunc(s.Entries, func(e localfs.Entry) bool { return e.Name == ".." })
}

func TestFilterSpecMatchScope(t *testing.T) {
	file := localfs.Entry{Name: "harbor.go", Type: localfs.EntryFile}
	dir := localfs.Entry{Name: "harbor.go", Type: localfs.EntryDirectory}
	for _, tt := range []struct {
		scope             PatternScope
		wantFile, wantDir bool
	}{
		{ScopeFiles, true, false},
		{ScopeDirs, false, true},
		{ScopeBoth, true, true},
	} {
		match, err := FilterSpec{Pattern: "*.go", Mode: GroupPatternShell, Scope: tt.scope}.MatchFunc()
		if err != nil {
			t.Fatal(err)
		}
		if got := match(file, GroupSelectMeta{}); got != tt.wantFile {
			t.Errorf("scope %d file match = %v, want %v", tt.scope, got, tt.wantFile)
		}
		if got := match(dir, GroupSelectMeta{}); got != tt.wantDir {
			t.Errorf("scope %d dir match = %v, want %v", tt.scope, got, tt.wantDir)
		}
	}
}

func TestFilterSpecLabelNamesNonDefaultScope(t *testing.T) {
	for _, tt := range []struct {
		spec FilterSpec
		want string
	}{
		{FilterSpec{Pattern: "*.go"}, "Filter: *.go"},
		{FilterSpec{Pattern: "2024*", Scope: ScopeDirs}, "Filter: dirs 2024*"},
		{FilterSpec{Pattern: "*.go", Scope: ScopeBoth, Invert: true}, "Filter: all not *.go"},
	} {
		if got := tt.spec.Label(); got != tt.want {
			t.Errorf("Label() = %q, want %q", got, tt.want)
		}
	}
}

func TestGitFiltersHideDirsWithoutMatches(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"harbor", "lantern"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	harbor, lantern := filepath.Join(dir, "harbor"), filepath.Join(dir, "lantern")
	tests := []struct {
		name   string
		filter *EntryFilter
		want   []string
	}{
		{"tracked", GitTrackedFilter(), []string{"lantern"}},
		{"staged", GitStagedFilter(), []string{"lantern"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := New(dir)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			state.GitColumnActive = true
			state.GitByPath = map[string]gitstatus.Cell{
				harbor:  {NoTracked: true},
				lantern: {HasStaged: true},
			}
			state.SetEntryFilter(tt.filter)
			var got []string
			for i := 0; i < state.VisibleEntryCount(); i++ {
				if e, _, ok := state.VisibleEntry(i); ok && e.Name != ".." {
					got = append(got, e.Name)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("visible = %v, want %v", got, tt.want)
			}
		})
	}
}

func visibleNames(s *State) []string {
	var out []string
	for i := 0; i < s.VisibleEntryCount(); i++ {
		if e, _, ok := s.VisibleEntry(i); ok {
			out = append(out, e.Name)
		}
	}
	return out
}

func setupTypeFilterState(t *testing.T) State {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"thicket.mkv", "thicket.txt", "lantern.jpg", "sesame.MKV"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestPatternFilterTypes(t *testing.T) {
	types := regexp.MustCompile(`(?i)\.(mkv|jpg)$`)
	cases := []struct {
		name  string
		spec  FilterSpec
		want  []string // visible besides ".."
		count int      // CountPatternMatches: entries the pattern matches, in scope
	}{
		{"types keeps dirs", FilterSpec{Types: types}, []string{"lantern.jpg", "meadow", "sesame.MKV", "thicket.mkv"}, 3},
		{"types and pattern", FilterSpec{Types: types, Pattern: "thicket", Mode: GroupPatternSimple}, []string{"meadow", "thicket.mkv"}, 1},
		{"invert", FilterSpec{Types: types, Invert: true}, []string{"meadow", "thicket.txt"}, 1},
		{"types both hides dirs", FilterSpec{Types: types, Scope: ScopeBoth}, []string{"lantern.jpg", "sesame.MKV", "thicket.mkv"}, 3},
		{"invert both", FilterSpec{Types: types, Invert: true, Scope: ScopeBoth}, []string{"meadow", "thicket.txt"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := setupTypeFilterState(t)
			f, err := PatternFilter(tc.spec, nil)
			if err != nil {
				t.Fatal(err)
			}
			state.SetEntryFilter(f)
			got := slices.DeleteFunc(visibleNames(&state), func(n string) bool { return n == ".." })
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("visible = %v, want %v", got, want)
			}
			files, dirs, err := state.CountPatternMatches(tc.spec, GroupSelectMeta{})
			if err != nil {
				t.Fatal(err)
			}
			if files+dirs != tc.count {
				t.Fatalf("CountPatternMatches = %d files %d dirs, want %d entries", files, dirs, tc.count)
			}
		})
	}
}

func TestFilterSpecLabel(t *testing.T) {
	if got := (FilterSpec{Pattern: "walnut*"}).Label(); got != "Filter: walnut*" {
		t.Fatalf("Label = %q", got)
	}
	if got := (FilterSpec{Pattern: "a", TypeLabel: "Videos, Audio", Invert: true}).Label(); got != "Filter: not a Videos, Audio" {
		t.Fatalf("Label = %q", got)
	}
}

func TestFilterSpecRegexCaseSensitivity(t *testing.T) {
	for _, tc := range []struct {
		caseSensitive bool
		want          bool
	}{{false, true}, {true, false}} {
		match, err := FilterSpec{Pattern: `^lantern\.txt$`, Mode: GroupPatternRegex, CaseSensitive: tc.caseSensitive}.MatchFunc()
		if err != nil {
			t.Fatal(err)
		}
		if got := match(localfs.Entry{Name: "Lantern.TXT"}, GroupSelectMeta{}); got != tc.want {
			t.Errorf("caseSensitive=%v: got %v, want %v", tc.caseSensitive, got, tc.want)
		}
	}
}

func TestPatternFilterTreeKeepsAncestorsOfMatches(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"meadow/thicket", "harbor"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"meadow/thicket/lantern.go", "meadow/sesame.txt", "harbor/walnut.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	// Rows: harbor, meadow; expand meadow, then its thicket child (synchronous in tests).
	for _, cursor := range []int{1, 2} {
		state.Cursor = cursor
		if err := state.ToggleTreeExpand(10); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		scope PatternScope
		want  []string
	}{
		// Both: a match keeps its ancestors, other directories hide.
		{ScopeBoth, []string{"meadow", "thicket", "lantern.go"}},
		// Files: every directory stays, expanded ones keep their rows even with no match inside.
		{ScopeFiles, []string{"harbor", "meadow", "thicket", "lantern.go"}},
	} {
		f, err := PatternFilter(FilterSpec{Pattern: "*.go", Mode: GroupPatternShell, Scope: tt.scope}, nil)
		if err != nil {
			t.Fatal(err)
		}
		state.SetEntryFilter(f)
		if got := visibleNames(&state); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("scope %d: visible = %v, want %v", tt.scope, got, tt.want)
		}
	}
}
