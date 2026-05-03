package panel

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestNewLoadsAbsolutePathAndEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if !filepath.IsAbs(state.Path) {
		t.Fatalf("Path = %q, want absolute path", state.Path)
	}
	if len(state.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(state.Entries))
	}
	if state.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0", state.Cursor)
	}
}

func TestLoadClampsCursorAfterReload(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 100

	if err := state.Load(dir); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if state.Cursor != 0 {
		t.Fatalf("Cursor = %d, want clamped to 0", state.Cursor)
	}
}

func TestLoadRejectsNonDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "initial.txt"))
	filePath := filepath.Join(dir, "file.txt")
	writeFile(t, filePath)

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := state.Load(filePath); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if state.Path != dir {
		t.Fatalf("Path = %q, want unchanged %q", state.Path, dir)
	}
}

func TestMoveKeepsCursorVisible(t *testing.T) {
	state := State{Entries: testEntries(10)}

	state.Move(4, 3)

	if state.Cursor != 4 {
		t.Fatalf("Cursor = %d, want 4", state.Cursor)
	}
	if state.ScrollOffset != 2 {
		t.Fatalf("ScrollOffset = %d, want 2", state.ScrollOffset)
	}

	state.Move(-3, 3)

	if state.Cursor != 1 {
		t.Fatalf("Cursor = %d, want 1", state.Cursor)
	}
	if state.ScrollOffset != 1 {
		t.Fatalf("ScrollOffset = %d, want 1", state.ScrollOffset)
	}
}

func TestPageTopAndBottomKeepCursorVisible(t *testing.T) {
	state := State{Entries: testEntries(10)}

	state.Page(1, 4)
	if state.Cursor != 4 || state.ScrollOffset != 1 {
		t.Fatalf("after page down: cursor=%d scroll=%d, want cursor=4 scroll=1", state.Cursor, state.ScrollOffset)
	}

	state.Bottom(4)
	if state.Cursor != 9 || state.ScrollOffset != 6 {
		t.Fatalf("after bottom: cursor=%d scroll=%d, want cursor=9 scroll=6", state.Cursor, state.ScrollOffset)
	}

	state.Top(4)
	if state.Cursor != 0 || state.ScrollOffset != 0 {
		t.Fatalf("after top: cursor=%d scroll=%d, want cursor=0 scroll=0", state.Cursor, state.ScrollOffset)
	}
}

func TestRefreshPreservesSelectedEntryByName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))
	writeFile(t, filepath.Join(dir, "c.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 1

	if err := state.Refresh(2); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	entry, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false, want true")
	}
	if entry.Name != "b.txt" {
		t.Fatalf("selected entry = %q, want b.txt", entry.Name)
	}
}

func TestToggleHiddenReloadsPanelEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden"))
	writeFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(state.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want only visible entry", len(state.Entries))
	}

	if err := state.ToggleHidden(5); err != nil {
		t.Fatalf("ToggleHidden() error = %v", err)
	}

	if !state.ShowHidden {
		t.Fatal("ShowHidden = false, want true")
	}
	if len(state.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want hidden and visible entries", len(state.Entries))
	}
}

func TestToggleHiddenPreservesCurrentEntryWhenStillVisible(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden"))
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 1

	if err := state.ToggleHidden(5); err != nil {
		t.Fatalf("ToggleHidden() error = %v", err)
	}

	entry, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false, want true")
	}
	if entry.Name != "b.txt" {
		t.Fatalf("selected entry = %q, want b.txt", entry.Name)
	}
}

func TestToggleSelectionSelectsAndUnselectsCurrentEntry(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "b.txt", Path: "/tmp/b.txt"},
		},
	}

	selected := state.ToggleSelection()
	if !selected {
		t.Fatal("ToggleSelection() selected = false, want true")
	}
	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("first entry is not selected after toggle")
	}

	selected = state.ToggleSelection()
	if selected {
		t.Fatal("ToggleSelection() selected = true, want false after second toggle")
	}
	if state.IsSelected(state.Entries[0]) {
		t.Fatal("first entry is selected after second toggle")
	}
}

func TestToggleSelectionWithEmptyEntriesIsInert(t *testing.T) {
	state := State{}

	if state.ToggleSelection() {
		t.Fatal("ToggleSelection() selected = true, want false for empty panel")
	}
	if len(state.SelectedPaths) != 0 {
		t.Fatalf("SelectedPaths = %v, want empty", state.SelectedPaths)
	}
}

func TestToggleSelectionAndAdvanceMovesToNextRow(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "b.txt", Path: "/tmp/b.txt"},
		},
	}

	toggled := state.ToggleSelectionAndAdvance(5)
	if !toggled {
		t.Fatal("ToggleSelectionAndAdvance() toggled = false, want true")
	}
	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("first entry is not selected after toggle")
	}
	if state.Cursor != 1 {
		t.Fatalf("Cursor = %d, want 1", state.Cursor)
	}
}

func TestToggleSelectionAndAdvanceStaysOnLastRow(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "b.txt", Path: "/tmp/b.txt"},
		},
		Cursor: 1,
	}

	toggled := state.ToggleSelectionAndAdvance(5)
	if !toggled {
		t.Fatal("ToggleSelectionAndAdvance() toggled = false, want true")
	}
	if !state.IsSelected(state.Entries[1]) {
		t.Fatal("last entry is not selected after toggle")
	}
	if state.Cursor != 1 {
		t.Fatalf("Cursor = %d, want to stay on last row", state.Cursor)
	}
}

func TestRefreshPreservesVisibleSelection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 1
	entry, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false, want true")
	}
	state.ToggleSelection()

	if err := state.Refresh(5); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !state.IsSelected(entry) {
		t.Fatalf("entry %q is not selected after refresh", entry.Name)
	}
}

func TestEnterDirectoryAndParentPreservesExitedDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	writeFile(t, filepath.Join(root, "z.txt"))

	state, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	entered, err := state.Enter(5)
	if err != nil {
		t.Fatalf("Enter() error = %v", err)
	}
	if !entered {
		t.Fatal("Enter() entered = false, want true")
	}
	if filepath.Base(state.Path) != "sub" {
		t.Fatalf("Path = %q, want sub directory", state.Path)
	}

	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}
	if state.Path != root {
		t.Fatalf("Path = %q, want %q", state.Path, root)
	}

	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "sub" {
		t.Fatalf("selected entry = %q ok=%v, want sub", entry.Name, ok)
	}
	subPath := filepath.Join(root, "sub")
	if len(state.History) < 2 || cleanPath(state.History[0]) != cleanPath(root) || cleanPath(state.History[1]) != cleanPath(subPath) {
		t.Fatalf("History = %v, want [%q %q] (MRU first)", state.History, root, subPath)
	}
	if state.HistoryIndex != 0 {
		t.Fatalf("HistoryIndex = %d, want 0", state.HistoryIndex)
	}
}

func TestHistoryBackwardReentersDirectoryLeftByParent(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	nested := filepath.Join(sub, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	entered, err := state.Enter(5)
	if err != nil || !entered {
		t.Fatalf("Enter sub: entered=%v err=%v", entered, err)
	}
	entered, err = state.Enter(5)
	if err != nil || !entered {
		t.Fatalf("Enter nested: entered=%v err=%v", entered, err)
	}

	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent from nested error = %v", err)
	}
	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent from sub error = %v", err)
	}
	if state.Path != root {
		t.Fatalf("Path = %q, want root %q", state.Path, root)
	}

	moved, err := state.HistoryBackward(5)
	if err != nil || !moved {
		t.Fatalf("HistoryBackward to sub: moved=%v err=%v", moved, err)
	}
	if state.Path != sub {
		t.Fatalf("Path = %q, want sub %q", state.Path, sub)
	}

	moved, err = state.HistoryBackward(5)
	if err != nil || !moved {
		t.Fatalf("HistoryBackward to nested: moved=%v err=%v", moved, err)
	}
	if state.Path != nested {
		t.Fatalf("Path = %q, want nested %q", state.Path, nested)
	}

	moved, err = state.HistoryBackward(5)
	if err != nil {
		t.Fatalf("HistoryBackward at end error = %v", err)
	}
	if moved {
		t.Fatal("HistoryBackward at end moved = true, want false")
	}
}

func TestEnterSiblingPrunesDescendantHistoryEntries(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	other := filepath.Join(root, "other")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir(sub) error = %v", err)
	}
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatalf("Mkdir(other) error = %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 1
	if _, err := state.Enter(5); err != nil {
		t.Fatalf("Enter(sub) error = %v", err)
	}
	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}
	if len(state.History) < 2 {
		t.Fatalf("History = %v, want parent visit recorded", state.History)
	}

	state.Cursor = 0
	if _, err := state.Enter(5); err != nil {
		t.Fatalf("Enter(other) error = %v", err)
	}
	subPath := filepath.Clean(sub)
	for _, p := range state.History {
		if filepath.Clean(p) == subPath {
			t.Fatalf("History = %v, should not retain exited subdirectory after sibling enter", state.History)
		}
	}
	if filepath.Clean(state.History[0]) != filepath.Clean(other) {
		t.Fatalf("History[0] = %q, want other dir %q", state.History[0], other)
	}
}

func TestEnterRegularFileIsInert(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	entered, err := state.Enter(5)
	if err != nil {
		t.Fatalf("Enter() error = %v", err)
	}
	if entered {
		t.Fatal("Enter() entered = true, want false")
	}
	if state.Path != dir {
		t.Fatalf("Path = %q, want unchanged %q", state.Path, dir)
	}
}

func TestQuickFilterKeepsEntriesVisibleAndMovesCursorToFirstVisibleMatch(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "notes.txt", Path: "/tmp/notes"},
			{Name: "src", Path: "/tmp/src"},
			{Name: "zzz.txt", Path: "/tmp/zzz"},
		},
		Cursor: 2,
		Filter: FilterState{CaseInsensitive: true},
	}

	state.OpenFilter(5)
	state.AppendFilterRune('s', 5)

	if state.VisibleEntryCount() != 3 {
		t.Fatalf("VisibleEntryCount() = %d, want all entries visible", state.VisibleEntryCount())
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "notes.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want first visible match notes.txt", entry.Name, ok)
	}
}

func TestQuickFilterMultiLetterSelectsBestRankedMatch(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "abzzc.txt", Path: "/tmp/abzzc"},
			{Name: "abc.txt", Path: "/tmp/abc"},
		},
		Filter: FilterState{CaseInsensitive: true},
	}

	state.OpenFilter(5)
	for _, r := range "abc" {
		state.AppendFilterRune(r, 5)
	}

	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "abc.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want best ranked match abc.txt", entry.Name, ok)
	}
}

func TestCycleFilterMatchStepsVisibleMatchesWithoutSkipping(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "notes.txt", Path: "/p/notes"},
			{Name: "alpha.txt", Path: "/p/alpha"},
			{Name: "src", Path: "/p/src"},
			{Name: "assets", Path: "/p/assets"},
		},
		Filter: FilterState{CaseInsensitive: true},
	}
	state.OpenFilter(5)
	state.AppendFilterRune('s', 5)

	first, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false")
	}
	if first.Name != "notes.txt" {
		t.Fatalf("CurrentEntry() = %q, want first visible match notes.txt", first.Name)
	}

	state.CycleFilterMatch(1, 5)
	second, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after down")
	}
	if second.Name != "src" {
		t.Fatalf("after down want next visible match src, got %q", second.Name)
	}

	state.CycleFilterMatch(1, 5)
	third, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after second down")
	}
	if third.Name != "assets" {
		t.Fatalf("after second down want next visible match assets, got %q", third.Name)
	}

	state.CycleFilterMatch(1, 5)
	wrapped, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after third down")
	}
	if wrapped.Name != first.Name {
		t.Fatalf("after down from last match want wrap to %q, got %q", first.Name, wrapped.Name)
	}

	state.CycleFilterMatch(-1, 5)
	backToLast, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after up from first")
	}
	if backToLast.Name != "assets" {
		t.Fatalf("after up from first match want wrap to assets, got %q", backToLast.Name)
	}
}

func TestCycleFilterMatchRankedOrderDiffersFromVisual(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "aaa_s.txt", Path: "/p/0"},
			{Name: "b.txt", Path: "/p/1"},
			{Name: "s.txt", Path: "/p/2"},
			{Name: "zzzzs.txt", Path: "/p/3"},
		},
		Filter: FilterState{
			CaseInsensitive: true,
			CycleMatches:    "ranked",
		},
	}
	state.OpenFilter(5)
	state.AppendFilterRune('s', 5)

	first, ok := state.CurrentEntry()
	if !ok || first.Name != "aaa_s.txt" {
		t.Fatalf("single-letter query should land on first visual match, got %q ok=%v", first.Name, ok)
	}

	state.CycleFilterMatch(1, 5)
	second, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after down")
	}
	if second.Name != "zzzzs.txt" {
		t.Fatalf("ranked cycle: after aaa_s want zzzzs.txt, got %q", second.Name)
	}

	state.Filter.CycleMatches = ""
	state.Cursor = 0 // aaa_s.txt
	state.CycleFilterMatch(1, 5)
	visualSecond, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after visual down")
	}
	if visualSecond.Name != "s.txt" {
		t.Fatalf("visual cycle: after aaa_s want s.txt, got %q", visualSecond.Name)
	}
}

func TestCycleFilterMatchWithNoMatchesFallsBackToMove(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "alpha.txt", Path: "/p/alpha"},
			{Name: "beta.txt", Path: "/p/beta"},
		},
		Cursor: 0,
		Filter: FilterState{CaseInsensitive: true},
	}
	state.OpenFilter(5)
	for _, r := range "zzz" {
		state.AppendFilterRune(r, 5)
	}
	if len(state.Filter.results) != 0 {
		t.Fatalf("want no matches for zzz, got %d", len(state.Filter.results))
	}
	state.CycleFilterMatch(1, 5)
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "beta.txt" {
		t.Fatalf("CurrentEntry() = %v ok=%v, want beta after Move fallback", entry.Name, ok)
	}
}

func TestQuickFilterSelectionUsesEntryPath(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "alpha.txt", Path: "/tmp/alpha"},
			{Name: "beta.txt", Path: "/tmp/beta"},
		},
		Filter: FilterState{CaseInsensitive: true},
	}

	state.OpenFilter(5)
	for _, r := range "beta" {
		state.AppendFilterRune(r, 5)
	}
	state.ToggleSelection()

	if !state.IsSelected(state.Entries[1]) {
		t.Fatal("beta entry is not selected after toggling filtered row")
	}
	if state.IsSelected(state.Entries[0]) {
		t.Fatal("alpha entry is selected, want only filtered beta row")
	}
}

func TestQuickFilterAcceptCancelAndClear(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "alpha.txt", Path: "/tmp/alpha"},
			{Name: "beta.txt", Path: "/tmp/beta"},
		},
		Filter: FilterState{CaseInsensitive: true},
	}

	state.OpenFilter(5)
	for _, r := range "beta" {
		state.AppendFilterRune(r, 5)
	}
	state.AcceptFilter(5)
	if state.Filter.Editing {
		t.Fatal("filter editing = true, want false after accept")
	}
	if !state.Filter.Active || state.VisibleEntryCount() != 2 {
		t.Fatalf("filter active=%v visible=%d, want accepted fuzzy query with all entries visible", state.Filter.Active, state.VisibleEntryCount())
	}

	state.OpenFilter(5)
	state.ClearFilter(5)
	if !state.Filter.Editing {
		t.Fatal("filter editing = false, want clear to keep editing")
	}
	if state.Filter.Active || state.VisibleEntryCount() != 2 {
		t.Fatalf("after clear active=%v visible=%d, want unfiltered entries", state.Filter.Active, state.VisibleEntryCount())
	}

	state.AppendFilterRune('a', 5)
	state.CancelFilter(5)
	if state.Filter.Editing || state.Filter.Active || state.Filter.Query != "" {
		t.Fatalf("after cancel editing=%v active=%v query=%q, want reset filter", state.Filter.Editing, state.Filter.Active, state.Filter.Query)
	}
	if state.VisibleEntryCount() != 2 {
		t.Fatalf("VisibleEntryCount() = %d, want restored unfiltered entries", state.VisibleEntryCount())
	}
}

// Sort tests

func TestApplySortByName(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "z.txt", Path: "/tmp/z.txt"},
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "m.txt", Path: "/tmp/m.txt"},
		},
	}
	state.Sort = SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	want := []string{"a.txt", "m.txt", "z.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortByNameReverse(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "z.txt", Path: "/tmp/z.txt"},
			{Name: "m.txt", Path: "/tmp/m.txt"},
		},
	}
	state.Sort = SortState{Mode: SortName, Reverse: true, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	want := []string{"z.txt", "m.txt", "a.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortByExtension(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.go", Path: "/tmp/a.go"},
			{Name: "b.txt", Path: "/tmp/b.txt"},
			{Name: "c.go", Path: "/tmp/c.go"},
			{Name: "d", Path: "/tmp/d"},
		},
	}
	state.Sort = SortState{Mode: SortExtension, Reverse: false, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	// no extension < .go < .txt
	want := []string{"d", "a.go", "c.go", "b.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortBySize(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "large.bin", Path: "/tmp/large.bin", Size: 1000},
			{Name: "small.txt", Path: "/tmp/small.txt", Size: 10},
			{Name: "medium.txt", Path: "/tmp/medium.txt", Size: 100},
		},
	}
	state.Sort = SortState{Mode: SortSize, Reverse: false, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	want := []string{"small.txt", "medium.txt", "large.bin"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortBySizeReverse(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "small.txt", Path: "/tmp/small.txt", Size: 10},
			{Name: "large.bin", Path: "/tmp/large.bin", Size: 1000},
			{Name: "medium.txt", Path: "/tmp/medium.txt", Size: 100},
		},
	}
	state.Sort = SortState{Mode: SortSize, Reverse: true, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	want := []string{"large.bin", "medium.txt", "small.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortByMtime(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "old.txt", Path: "/tmp/old.txt"},
			{Name: "new.txt", Path: "/tmp/new.txt"},
			{Name: "mid.txt", Path: "/tmp/mid.txt"},
		},
	}
	// Set different mtimes
	state.Entries[0].ModifiedAt = mustParseTime("2020-01-01T00:00:00Z")
	state.Entries[1].ModifiedAt = mustParseTime("2024-01-01T00:00:00Z")
	state.Entries[2].ModifiedAt = mustParseTime("2022-01-01T00:00:00Z")

	state.Sort = SortState{Mode: SortMtime, Reverse: false, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	want := []string{"old.txt", "mid.txt", "new.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortDirectoriesFirst(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "b.txt", Path: "/tmp/b.txt", Type: localfs.EntryFile},
			{Name: "a-dir", Path: "/tmp/a-dir", Type: localfs.EntryDirectory},
			{Name: "z-dir", Path: "/tmp/z-dir", Type: localfs.EntryDirectory},
			{Name: "a.txt", Path: "/tmp/a.txt", Type: localfs.EntryFile},
		},
	}
	state.Sort = SortState{Mode: SortName, Reverse: false, DirectoriesFirst: true}
	state.ApplySort()

	names := entryNames(state.Entries)
	want := []string{"a-dir", "z-dir", "a.txt", "b.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortDirectoriesFirstFalse(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "z-dir", Path: "/tmp/z-dir", Type: localfs.EntryDirectory},
			{Name: "a.txt", Path: "/tmp/a.txt", Type: localfs.EntryFile},
		},
	}
	state.Sort = SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	want := []string{"a.txt", "z-dir"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestApplySortDeterministicTieBreakByName(t *testing.T) {
	// Same name, different case and paths should find a stable order
	state := State{
		Entries: []localfs.Entry{
			{Name: "File.txt", Path: "/tmp/File.txt", Type: localfs.EntryFile},
			{Name: "file.txt", Path: "/tmp/file.txt", Type: localfs.EntryFile},
		},
	}
	state.Sort = SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false}
	state.ApplySort()

	names := entryNames(state.Entries)
	// case-insensitive: both same, so original case determines: F < f
	want := []string{"File.txt", "file.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestSetSortModePreservesCursorByPath(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "z.txt", Path: "/tmp/z.txt"},
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "m.txt", Path: "/tmp/m.txt"},
		},
		Cursor: 0, // on z.txt
	}
	state.Sort = SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false}
	state.SetSortMode(SortName, false, false, 5)

	// After sorting, cursor should be on z.txt (preserved by name) at index 2
	if state.Cursor != 2 {
		t.Fatalf("Cursor = %d, want 2 (position of z.txt after ascending sort)", state.Cursor)
	}
	entry, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false, want true")
	}
	if entry.Name != "z.txt" {
		t.Fatalf("CurrentEntry() = %q, want z.txt preserved by path", entry.Name)
	}
}

func TestInvertSelection(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "b.txt", Path: "/tmp/b.txt"},
			{Name: "c.txt", Path: "/tmp/c.txt"},
		},
		SelectedPaths: map[string]bool{"/tmp/a.txt": true},
	}

	state.InvertSelection()

	if state.IsSelected(state.Entries[0]) {
		t.Fatal("a.txt should no longer be selected after invert")
	}
	if !state.IsSelected(state.Entries[1]) {
		t.Fatal("b.txt should be selected after invert")
	}
	if !state.IsSelected(state.Entries[2]) {
		t.Fatal("c.txt should be selected after invert")
	}
}

func TestInvertSelectionWithEmptyPreservesNil(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
		},
	}

	state.InvertSelection()
	if state.SelectedPaths == nil {
		t.Fatal("SelectedPaths should not be nil after invert")
	}
	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("a.txt should be selected after invert from empty")
	}

	state.InvertSelection()
	if state.SelectedPaths != nil {
		t.Fatal("SelectedPaths should be nil after second invert (all deselected)")
	}
}

func TestClearSelection(t *testing.T) {
	state := State{
		Path: "/tmp",
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "b.txt", Path: "/tmp/b.txt"},
		},
		SelectedPaths: map[string]bool{"/tmp/a.txt": true, "/tmp/b.txt": true},
	}

	state.ClearSelection()
	if state.SelectedPaths != nil {
		t.Fatal("SelectedPaths should be nil after clear (all paths were under current directory)")
	}
	if state.IsSelected(state.Entries[0]) {
		t.Fatal("a.txt should not be selected after clear")
	}
}

func TestClearSelectionLeavesOtherDirectoriesSelected(t *testing.T) {
	state := State{
		Path: "/tmp/here",
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/here/a.txt"},
		},
		SelectedPaths: map[string]bool{
			"/tmp/here/a.txt":  true,
			"/tmp/other/b.txt": true,
		},
	}

	state.ClearSelection()
	if state.SelectedPaths == nil {
		t.Fatal("SelectedPaths should keep off-directory selection")
	}
	if state.SelectedPaths["/tmp/other/b.txt"] != true {
		t.Fatal("off-directory path should stay selected")
	}
	if state.SelectedPaths["/tmp/here/a.txt"] {
		t.Fatal("current-directory path should be cleared")
	}
}

func TestSelectGroupByGlob(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "main.go", Path: "/tmp/main.go"},
			{Name: "main_test.go", Path: "/tmp/main_test.go"},
			{Name: "utils.go", Path: "/tmp/utils.go"},
			{Name: "README.md", Path: "/tmp/README.md"},
		},
	}

	state.SelectGroup("*.go", false, false, true)

	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("main.go should be selected")
	}
	if !state.IsSelected(state.Entries[1]) {
		t.Fatal("main_test.go should be selected")
	}
	if !state.IsSelected(state.Entries[2]) {
		t.Fatal("utils.go should be selected")
	}
	if state.IsSelected(state.Entries[3]) {
		t.Fatal("README.md should not be selected")
	}
}

func TestUnselectGroupByGlob(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "main.go", Path: "/tmp/main.go"},
			{Name: "utils.go", Path: "/tmp/utils.go"},
			{Name: "README.md", Path: "/tmp/README.md"},
		},
		SelectedPaths: map[string]bool{
			"/tmp/main.go":   true,
			"/tmp/utils.go":  true,
			"/tmp/README.md": true,
		},
	}

	state.UnselectGroup("*.go", false, false, true)

	if state.IsSelected(state.Entries[0]) {
		t.Fatal("main.go should be unselected")
	}
	if state.IsSelected(state.Entries[1]) {
		t.Fatal("utils.go should be unselected")
	}
	if !state.IsSelected(state.Entries[2]) {
		t.Fatal("README.md should remain selected")
	}
}

func TestSelectGroupFilesOnly(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "main.go", Path: "/tmp/main.go", Type: localfs.EntryFile},
			{Name: "src", Path: "/tmp/src", Type: localfs.EntryDirectory},
			{Name: "utils.go", Path: "/tmp/utils.go", Type: localfs.EntryFile},
		},
	}

	state.SelectGroup("*", true, false, true)

	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("main.go should be selected")
	}
	if state.IsSelected(state.Entries[1]) {
		t.Fatal("src directory should NOT be selected with filesOnly")
	}
	if !state.IsSelected(state.Entries[2]) {
		t.Fatal("utils.go should be selected")
	}
}

func TestSelectGroupSubstringCaseSensitive(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "main.go", Path: "/tmp/main.go"},
			{Name: "Main_test.go", Path: "/tmp/Main_test.go"},
			{Name: "README.md", Path: "/tmp/README.md"},
		},
	}

	// Substring match, case-sensitive
	state.SelectGroup("main", false, true, false)

	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("main.go should be selected (case-sensitive substring match)")
	}
	if state.IsSelected(state.Entries[1]) {
		t.Fatal("Main_test.go should NOT be selected (case-sensitive)")
	}
	if state.IsSelected(state.Entries[2]) {
		t.Fatal("README.md should not be selected")
	}
}

func TestSelectGroupSubstringCaseInsensitive(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "main.go", Path: "/tmp/main.go"},
			{Name: "Main_test.go", Path: "/tmp/Main_test.go"},
			{Name: "README.md", Path: "/tmp/README.md"},
		},
	}

	// Substring match, case-insensitive
	state.SelectGroup("main", false, false, false)

	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("main.go should be selected (case-insensitive substring)")
	}
	if !state.IsSelected(state.Entries[1]) {
		t.Fatal("Main_test.go should be selected (case-insensitive substring)")
	}
	if state.IsSelected(state.Entries[2]) {
		t.Fatal("README.md should not be selected")
	}
}

func TestCycleSort(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "b.go", Path: "/tmp/b.go"},
		},
		Sort: SortState{Mode: SortName, DirectoriesFirst: false},
	}

	state.CycleSort(5)
	if state.Sort.Mode != SortExtension {
		t.Fatalf("SortMode after cycle = %v, want SortExtension", state.Sort.Mode)
	}

	state.CycleSort(5)
	if state.Sort.Mode != SortSize {
		t.Fatalf("SortMode after second cycle = %v, want SortSize", state.Sort.Mode)
	}

	state.CycleSort(5)
	if state.Sort.Mode != SortMtime {
		t.Fatalf("SortMode after third cycle = %v, want SortMtime", state.Sort.Mode)
	}

	state.CycleSort(5)
	if state.Sort.Mode != SortName {
		t.Fatalf("SortMode after fourth cycle = %v, want SortName", state.Sort.Mode)
	}

	state.CycleSort(5)
	if state.Sort.Mode != SortExtension {
		t.Fatalf("SortMode after fifth cycle = %v, want SortExtension", state.Sort.Mode)
	}
}

func TestLoadAppliesDiskTotalsSortImmediatelyWhenListingFullyCached(t *testing.T) {
	dir := t.TempDir()
	smallDir := filepath.Join(dir, "aaa")
	largeDir := filepath.Join(dir, "zzz")
	if err := os.Mkdir(smallDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(largeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	state := State{
		ShowHidden: false,
		Filter:     FilterState{CaseInsensitive: true},
		Sort: SortState{
			Mode:                  SortName,
			Reverse:               false,
			DirectoriesFirst:      false,
			DiskUsageIdleSizeSort: true,
		},
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		c := filepath.Clean(abs)
		switch c {
		case filepath.Clean(smallDir):
			return 10, true
		case filepath.Clean(largeDir):
			return 9999, true
		default:
			return 0, false
		}
	}

	if err := state.load(dir, "", 10); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !state.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be true when listing is fully cached on load")
	}
	if len(state.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(state.Entries))
	}
	if state.Entries[0].Name != "zzz" {
		t.Fatalf("first entry = %q, want zzz (larger disk total sorts first)", state.Entries[0].Name)
	}
}

func TestIdleDiskTotalsSortLargestFirst(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "small.bin", Path: "/tmp/small.bin", Size: 999},
			{Name: "huge.bin", Path: "/tmp/huge.bin", Size: 1},
		},
		Sort:               SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false, DiskUsageIdleSizeSort: true},
		IdleDiskTotalsSort: true,
	}
	cache := map[string]int64{
		"/tmp/small.bin": 100,
		"/tmp/huge.bin":  8000,
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		v, ok := cache[filepath.Clean(abs)]
		return v, ok
	}
	state.ApplySort()
	if state.Entries[0].Name != "huge.bin" || state.Entries[1].Name != "small.bin" {
		t.Fatalf("got %q before %q, want huge before small for idle disk totals sort", state.Entries[0].Name, state.Entries[1].Name)
	}
}

func TestIdleDiskTotalsSortIgnoresSortReverse(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "small.bin", Path: "/tmp/small.bin", Size: 999},
			{Name: "huge.bin", Path: "/tmp/huge.bin", Size: 1},
		},
		Sort:               SortState{Mode: SortName, Reverse: true, DirectoriesFirst: false, DiskUsageIdleSizeSort: true},
		IdleDiskTotalsSort: true,
	}
	cache := map[string]int64{
		"/tmp/small.bin": 100,
		"/tmp/huge.bin":  8000,
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		v, ok := cache[filepath.Clean(abs)]
		return v, ok
	}
	state.ApplySort()
	if state.Entries[0].Name != "huge.bin" || state.Entries[1].Name != "small.bin" {
		t.Fatalf("idle disk totals largest-first should ignore Sort.Reverse: got %v then %v", state.Entries[0].Name, state.Entries[1].Name)
	}
}

func TestIdleDiskTotalsSortUnknownLast(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "known.bin", Path: "/tmp/known.bin"},
			{Name: "unknown.bin", Path: "/tmp/unknown.bin"},
		},
		Sort:               SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false, DiskUsageIdleSizeSort: true},
		IdleDiskTotalsSort: true,
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		if filepath.Clean(abs) == "/tmp/known.bin" {
			return 500, true
		}
		return 0, false
	}
	state.ApplySort()
	if state.Entries[0].Name != "known.bin" || state.Entries[1].Name != "unknown.bin" {
		t.Fatalf("got %q before %q, want cached path before unknown", state.Entries[0].Name, state.Entries[1].Name)
	}
}

func TestRefreshDiskUsageOrderingKeepsCursor(t *testing.T) {
	pathA := filepath.Join("/tmp", "a.bin")
	pathB := filepath.Join("/tmp", "b.bin")
	sizes := map[string]int64{
		filepath.Clean(pathA): 100,
		filepath.Clean(pathB): 10,
	}
	state := State{
		Path: "/tmp",
		Entries: []localfs.Entry{
			{Name: "a.bin", Path: pathA},
			{Name: "b.bin", Path: pathB},
		},
		Sort:               SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false, DiskUsageIdleSizeSort: true},
		IdleDiskTotalsSort: true,
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		v, ok := sizes[filepath.Clean(abs)]
		return v, ok
	}
	state.ApplySort()
	if state.Entries[0].Name != "a.bin" || state.Entries[1].Name != "b.bin" {
		t.Fatalf("initial order want a.bin (larger) first, got %v then %v", state.Entries[0].Name, state.Entries[1].Name)
	}
	state.Cursor = 1
	ent, ok := state.CurrentEntry()
	if !ok || ent.Name != "b.bin" {
		k := ""
		if ok {
			k = ent.Name
		}
		t.Fatalf("CurrentEntry with cursor=1 = %q, ok=%v want b.bin", k, ok)
	}
	sizes[filepath.Clean(pathA)] = 1
	sizes[filepath.Clean(pathB)] = 1000
	state.RefreshDiskUsageOrdering(5, false)
	if state.Entries[0].Name != "b.bin" || state.Entries[1].Name != "a.bin" {
		t.Fatalf("got order %v / %v after refresh, want b.bin then a.bin", state.Entries[0].Name, state.Entries[1].Name)
	}
	ent, ok = state.CurrentEntry()
	if !ok || ent.Name != "b.bin" {
		k := ""
		if ok {
			k = ent.Name
		}
		t.Fatalf("cursor name = %q ok=%v, want preserved b.bin", k, ok)
	}
}

func TestEnsureCursorCentered(t *testing.T) {
	entries := make([]localfs.Entry, 20)
	for i := range entries {
		name := strconv.Itoa(i) + ".txt"
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:    "/tmp",
		Entries: entries,
		Sort:    SortState{Mode: SortName, DirectoriesFirst: false},
	}
	state.ApplySort()
	state.Cursor = 10
	state.EnsureCursorCentered(5)
	if state.Cursor != 10 {
		t.Fatalf("Cursor = %d, want 10", state.Cursor)
	}
	if state.ScrollOffset != 8 {
		t.Fatalf("ScrollOffset = %d, want 8", state.ScrollOffset)
	}
}

func TestRefreshDiskUsageOrderingCentersCursorWhenRequested(t *testing.T) {
	entries := make([]localfs.Entry, 15)
	sizes := map[string]int64{}
	for i := range entries {
		name := strconv.Itoa(i) + ".dat"
		p := filepath.Join("/tmp", name)
		entries[i] = localfs.Entry{Name: name, Path: p}
		sizes[p] = int64(i)
	}
	state := State{
		Path:               "/tmp",
		Entries:            entries,
		Sort:               SortState{Mode: SortName, DirectoriesFirst: false, DiskUsageIdleSizeSort: true},
		IdleDiskTotalsSort: true,
		Cursor:             7,
		ScrollOffset:       0,
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		v, ok := sizes[filepath.Clean(abs)]
		return v, ok
	}
	state.RefreshDiskUsageOrdering(5, true)
	ent, ok := state.CurrentEntry()
	if !ok || ent.Name != "7.dat" {
		name := ""
		if ok {
			name = ent.Name
		}
		t.Fatalf("cursor entry = %q ok=%v want 7.dat", name, ok)
	}
	row := state.Cursor - state.ScrollOffset
	if row != 2 {
		t.Fatalf("cursor viewport row = %d, want 2 (centered in 5 rows)", row)
	}
}

func TestListingFullyDiskCached(t *testing.T) {
	cache := map[string]int64{
		filepath.Clean("/tmp/a"): 1,
		filepath.Clean("/tmp/b"): 2,
	}
	s := State{
		Path: "/tmp",
		Entries: []localfs.Entry{
			{Name: "a", Path: "/tmp/a"},
			{Name: "b", Path: "/tmp/b"},
		},
		DiskSorter: func(abs string) (int64, bool) {
			v, ok := cache[filepath.Clean(abs)]
			return v, ok
		},
	}
	if !s.ListingFullyDiskCached() {
		t.Fatal("expected fully cached")
	}
	partial := State{
		Path: "/tmp",
		Entries: []localfs.Entry{
			{Name: "a", Path: "/tmp/a"},
			{Name: "b", Path: "/tmp/b"},
		},
		DiskSorter: func(abs string) (int64, bool) {
			if filepath.Clean(abs) == filepath.Clean("/tmp/a") {
				return 1, true
			}
			return 0, false
		},
	}
	if partial.ListingFullyDiskCached() {
		t.Fatal("expected not fully cached when one path missing from DiskSorter")
	}
}

func TestToggleSortReverse(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "z.txt", Path: "/tmp/z.txt"},
		},
		Sort: SortState{Mode: SortName, Reverse: false, DirectoriesFirst: false},
	}

	state.ToggleSortReverse(5)
	if !state.Sort.Reverse {
		t.Fatal("SortReverse should be true after toggle")
	}

	state.ToggleSortReverse(5)
	if state.Sort.Reverse {
		t.Fatal("SortReverse should be false after second toggle")
	}
}

func TestQuickFilterStillWorksAfterSortChange(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "alpha.txt", Path: "/tmp/alpha.txt"},
			{Name: "abeta.txt", Path: "/tmp/abeta.txt"},
			{Name: "xylophone.txt", Path: "/tmp/xylophone.txt"},
		},
		Sort:   SortState{Mode: SortName, DirectoriesFirst: false},
		Filter: FilterState{CaseInsensitive: true},
	}

	state.ApplySort()
	state.OpenFilter(5)
	state.AppendFilterRune('a', 5)

	if !state.Filter.Active {
		t.Fatal("Filter should be active after typing")
	}
	if len(state.Filter.results) != 2 {
		t.Fatalf("Filter results = %d, want 2 matching 'a'", len(state.Filter.results))
	}
	entry, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false")
	}
	if entry.Name != "alpha.txt" && entry.Name != "abeta.txt" {
		t.Fatalf("CurrentEntry() = %q, want an 'a'-matching entry", entry.Name)
	}

	// Change sort mode and verify filter still works
	state.SetSortMode(SortSize, false, false, 5)
	if !state.Filter.Active {
		t.Fatal("Filter should remain active after sort change")
	}
	if len(state.Filter.results) != 2 {
		t.Fatalf("Filter results after sort change = %d, want 2", len(state.Filter.results))
	}
	expectedNames := map[string]bool{"alpha.txt": true, "abeta.txt": true}
	for _, res := range state.Filter.results {
		name := state.Entries[res.Index].Name
		if !expectedNames[name] {
			t.Fatalf("Filter result name = %q, want only alpha.txt or abeta.txt", name)
		}
	}
}

func TestHasSelectionInSubtree(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(sub, "inner.txt")
	if err := os.WriteFile(inner, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := State{
		SelectedPaths: map[string]bool{
			inner: true,
		},
	}
	if !s.HasSelectionInSubtree(root) {
		t.Fatal("root should have subtree selection")
	}
	if !s.HasSelectionInSubtree(sub) {
		t.Fatal("sub should have subtree selection")
	}
	if s.HasSelectionInSubtree(inner) {
		t.Fatal("file path should not report subtree selection for itself")
	}
	other := filepath.Join(root, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if s.HasSelectionInSubtree(other) {
		t.Fatal("sibling dir should not have subtree selection")
	}
	// Selecting the directory entry path itself is not a "nested" selection for that row's marker.
	sOnly := State{SelectedPaths: map[string]bool{sub: true}}
	if sOnly.HasSelectionInSubtree(sub) {
		t.Fatal("row for sub/ should not get subtree marker when only that folder is selected")
	}
}

func TestCrossDirectorySelectionsAndStripOrder(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(root, "root.txt"))
	writeFile(t, filepath.Join(sub, "sub.txt"))

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Select root.txt
	for i := 0; i < state.VisibleEntryCount(); i++ {
		e, _, ok := state.VisibleEntry(i)
		if ok && e.Name == "root.txt" {
			state.Cursor = i
			break
		}
	}
	state.ToggleSelection()

	// Enter sub/
	for i := 0; i < state.VisibleEntryCount(); i++ {
		e, _, ok := state.VisibleEntry(i)
		if ok && e.Name == "sub" && e.Type == localfs.EntryDirectory {
			state.Cursor = i
			break
		}
	}
	entered, err := state.Enter(5)
	if err != nil || !entered {
		t.Fatalf("Enter sub: err=%v entered=%v", err, entered)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		e, _, ok := state.VisibleEntry(i)
		if ok && e.Name == "sub.txt" {
			state.Cursor = i
			break
		}
	}
	state.ToggleSelection()

	if !state.IsSelected(state.Entries[state.Cursor]) {
		t.Fatal("sub.txt should be selected")
	}

	rootEntryPath := filepath.Join(root, "root.txt")
	if !state.SelectedPaths[rootEntryPath] {
		t.Fatal("root.txt should still be selected after navigating to sub")
	}

	subTxtPath := filepath.Join(sub, "sub.txt")

	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent: %v", err)
	}
	// Left sub: sub.txt should appear in strip paths
	paths := state.SelectionsStripPaths()
	if len(paths) != 1 || paths[0] != subTxtPath {
		t.Fatalf("SelectionsStripPaths = %v, want single sub.txt path", paths)
	}

	// Re-enter sub: strip should drop sub.txt from order display (still selected in list)
	for i := 0; i < state.VisibleEntryCount(); i++ {
		e, _, ok := state.VisibleEntry(i)
		if ok && e.Name == "sub" && e.Type == localfs.EntryDirectory {
			state.Cursor = i
			break
		}
	}
	entered, err = state.Enter(5)
	if err != nil || !entered {
		t.Fatalf("Enter sub again: err=%v entered=%v", err, entered)
	}
	if len(state.SelectionsStripPaths()) != 1 || state.SelectionsStripPaths()[0] != rootEntryPath {
		t.Fatalf("strip should list root.txt while in sub, got %v", state.SelectionsStripPaths())
	}
	var subEntry localfs.Entry
	for _, e := range state.Entries {
		if e.Path == subTxtPath {
			subEntry = e
			break
		}
	}
	if !state.IsSelected(subEntry) {
		t.Fatal("sub.txt should still show selected in file list")
	}
}

func mustParseTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}

func entryNames(entries []localfs.Entry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names
}

func testEntries(count int) []localfs.Entry {
	entries := make([]localfs.Entry, count)
	for i := range entries {
		entries[i] = localfs.Entry{Name: string(rune('a' + i))}
	}
	return entries
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
