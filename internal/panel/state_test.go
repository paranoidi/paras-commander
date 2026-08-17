package panel

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/testutil"
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

	if !filepath.IsAbs(state.Path.String()) {
		t.Fatalf("Path = %q, want absolute path", state.Path)
	}
	if len(state.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(state.Entries))
	}
	if state.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0", state.Cursor)
	}
}

func TestRefreshVolumeSpacePreservesListing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	n := len(state.Entries)
	cur := state.Cursor
	okBefore := state.VolumeSpaceOK

	state.RefreshVolumeSpace()

	if len(state.Entries) != n {
		t.Fatalf("len(Entries) after RefreshVolumeSpace = %d, want %d", len(state.Entries), n)
	}
	if state.Cursor != cur {
		t.Fatalf("Cursor after RefreshVolumeSpace = %d, want %d", state.Cursor, cur)
	}
	if state.VolumeSpaceOK != okBefore {
		t.Fatalf("VolumeSpaceOK flipped from %v to %v (unexpected)", okBefore, state.VolumeSpaceOK)
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

func TestRefreshRestoresCursorByIndexWhenCurrentEntryRemoved(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "a.txt"))
	bPath := filepath.Join(dir, "b.txt")
	testutil.WriteFile(t, bPath)
	testutil.WriteFile(t, filepath.Join(dir, "c.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 1
	if name, ok := state.CurrentEntry(); !ok || name.Name != "b.txt" {
		t.Fatalf("precondition cursor entry = %v ok=%v, want b.txt", name, ok)
	}
	if err := os.Remove(bPath); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := state.Refresh(10); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if state.Cursor != 1 {
		t.Fatalf("Cursor = %d, want 1 (same row index after middle file removed)", state.Cursor)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "c.txt" {
		t.Fatalf("CurrentEntry = %v ok=%v, want c.txt under cursor", entry, ok)
	}
}

func TestRefreshPreservesScrollWhenCursorIndexUnchanged(t *testing.T) {
	const viewportRows = 5
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d.dat", i)
		testutil.WriteFile(t, filepath.Join(dir, name))
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.ScrollMode = ScrollModeMinimal
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "10.dat" {
			state.Cursor = i
			break
		}
	}
	state.ScrollOffset = 8
	priorScroll := state.ScrollOffset
	priorRow := state.Cursor - state.ScrollOffset
	entry, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false")
	}
	if err := os.Remove(entry.Path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := state.Refresh(viewportRows); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if state.ScrollOffset != priorScroll {
		t.Fatalf("ScrollOffset = %d, want %d (preserved)", state.ScrollOffset, priorScroll)
	}
	if row := state.Cursor - state.ScrollOffset; row != priorRow {
		t.Fatalf("cursor row in viewport = %d, want %d", row, priorRow)
	}
}

func TestRefreshAdjustsScrollWhenListShrinksBelowPriorOffset(t *testing.T) {
	const viewportRows = 5
	dir := t.TempDir()
	paths := make([]string, 20)
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d.dat", i)
		p := filepath.Join(dir, name)
		testutil.WriteFile(t, p)
		paths[i] = p
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.ScrollMode = ScrollModeMinimal
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "10.dat" {
			state.Cursor = i
			break
		}
	}
	state.ScrollOffset = 8
	for i := 11; i < 20; i++ {
		if err := os.Remove(paths[i]); err != nil {
			t.Fatalf("Remove: %v", err)
		}
	}
	if err := state.Refresh(viewportRows); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	wantScroll := clampScrollKeepingCursorVisible(8, state.Cursor, viewportRows, state.VisibleEntryCount())
	if state.ScrollOffset != wantScroll {
		t.Fatalf("ScrollOffset = %d, want %d (clamped, not reset to 0)", state.ScrollOffset, wantScroll)
	}
	if state.ScrollOffset == 0 && wantScroll != 0 {
		t.Fatal("ScrollOffset reset to 0 when prior offset should be preserved/clamped")
	}
}

func TestLoadRejectsNonDirectory(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "initial.txt"))
	filePath := filepath.Join(dir, "file.txt")
	testutil.WriteFile(t, filePath)

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := state.Load(filePath); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if state.Path.String() != dir {
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
	testutil.WriteFile(t, filepath.Join(dir, "a.txt"))
	testutil.WriteFile(t, filepath.Join(dir, "b.txt"))
	testutil.WriteFile(t, filepath.Join(dir, "c.txt"))

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

func TestRefreshOrNavigateToExistingAncestorWalksUpOnce(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	child := filepath.Join(parent, "child")
	grandchild := filepath.Join(child, "grandchild")
	for _, p := range []string{parent, child, grandchild} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state, err := New(grandchild)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}
	if err := state.RefreshOrNavigateToExistingAncestor(5); err != nil {
		t.Fatalf("RefreshOrNavigateToExistingAncestor() error = %v", err)
	}
	want := filepath.Clean(parent)
	if got := state.Path.String(); got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

func TestRefreshOrNavigateToExistingAncestorRefreshesWhenPathExists(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "keep.txt"))
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 0
	if err := state.RefreshOrNavigateToExistingAncestor(5); err != nil {
		t.Fatalf("RefreshOrNavigateToExistingAncestor() error = %v", err)
	}
	if state.Path.String() != filepath.Clean(dir) {
		t.Fatalf("Path = %q, want %q", state.Path.String(), dir)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "keep.txt" {
		t.Fatalf("CurrentEntry() = %v, want keep.txt", entry)
	}
}

func TestLoadHidesGitignoredEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, filepath.Join(dir, "ignored.txt"))
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	cache := gitignore.NewCache()
	state, err := NewWithOptions(dir, localfs.ListOptions{ShowHidden: false}, cache)
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if len(state.Entries) != 1 || state.Entries[0].Name != "visible.txt" {
		t.Fatalf("entries = %v, want only visible.txt", entryNames(state.Entries))
	}
	if !state.GitignoreActive {
		t.Fatal("GitignoreActive = false, want true inside Git work tree on first load")
	}
}

func TestLoadSetsGitColumnActiveInsideWorkTree(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFileBytes(t, filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"))
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.ScheduleGitStatus = func(GitStatusRequest) bool { return true }
	if err := state.Refresh(5); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !state.GitColumnActive {
		t.Fatal("GitColumnActive = false, want true inside Git work tree")
	}
	if !state.GitPending {
		t.Fatal("GitPending = false, want true before async status completes")
	}
	if state.GitByPath != nil {
		t.Fatal("GitByPath should be nil until async status completes")
	}
}

func TestLoadClearsGitColumnForInvalidGitMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.ScheduleGitStatus = func(GitStatusRequest) bool { return true }
	if err := state.Refresh(5); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if state.GitColumnActive {
		t.Fatal("GitColumnActive = true, want false when .git has no HEAD")
	}
	if state.GitPending {
		t.Fatal("GitPending = true, want false when git column inactive")
	}
}

func TestLoadClearsGitColumnOutsideWorkTree(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := state.Refresh(5); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if state.GitColumnActive {
		t.Fatal("GitColumnActive = true, want false outside Git work tree")
	}
}

func TestLoadClearsGitignoreActiveOutsideWorkTree(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Gitignore = gitignore.NewCache()
	if err := state.Refresh(5); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if state.GitignoreActive {
		t.Fatal("GitignoreActive = true, want false outside Git work tree")
	}
}

func TestLoadSetsDotfilesHiddenActiveWhenDotfilesPresent(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".hidden"))
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !state.DotfilesHiddenActive {
		t.Fatal("DotfilesHiddenActive = false, want true when dotfiles are hidden and present")
	}
}

func TestLoadClearsDotfilesHiddenActiveWhenNoDotfiles(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if state.DotfilesHiddenActive {
		t.Fatal("DotfilesHiddenActive = true, want false when directory has no dotfiles")
	}
}

func TestToggleHiddenReloadsPanelEntries(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".hidden"))
	testutil.WriteFile(t, filepath.Join(dir, "visible.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(state.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want only visible entry", len(state.Entries))
	}

	if err := state.SetShowHidden(true, 5); err != nil {
		t.Fatalf("SetShowHidden() error = %v", err)
	}

	if !state.ShowHidden {
		t.Fatal("ShowHidden = false, want true")
	}
	if state.DotfilesHiddenActive {
		t.Fatal("DotfilesHiddenActive = true, want false when show hidden is on")
	}
	if len(state.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want hidden and visible entries", len(state.Entries))
	}
}

func TestToggleHiddenPreservesCurrentEntryWhenStillVisible(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".hidden"))
	testutil.WriteFile(t, filepath.Join(dir, "a.txt"))
	testutil.WriteFile(t, filepath.Join(dir, "b.txt"))

	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	state.Cursor = 1

	if err := state.SetShowHidden(true, 5); err != nil {
		t.Fatalf("SetShowHidden() error = %v", err)
	}

	entry, ok := state.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false, want true")
	}
	if entry.Name != "b.txt" {
		t.Fatalf("selected entry = %q, want b.txt", entry.Name)
	}
}

func TestAddSelectionMarksPath(t *testing.T) {
	state := State{Path: pathloc.MustParse("/tmp")}
	state.AddSelection("/tmp/a.txt")
	if state.SelectedPaths == nil || !state.SelectedPaths["/tmp/a.txt"] {
		t.Fatal("AddSelection did not mark path")
	}
	state.AddSelection("/tmp/a.txt")
	if len(state.SelectedPaths) != 1 {
		t.Fatalf("duplicate AddSelection grew map: %v", state.SelectedPaths)
	}
}

func TestToggleSelectionSelectsAndUnselectsCurrentEntry(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt"},
			{Name: "b.txt", Path: "/tmp/b.txt"},
		},
	}

	selected, _ := state.ToggleSelection()
	if !selected {
		t.Fatal("ToggleSelection() selected = false, want true")
	}
	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("first entry is not selected after toggle")
	}

	selected, _ = state.ToggleSelection()
	if selected {
		t.Fatal("ToggleSelection() selected = true, want false after second toggle")
	}
	if state.IsSelected(state.Entries[0]) {
		t.Fatal("first entry is selected after second toggle")
	}
}

func TestToggleSelectionWithEmptyEntriesIsInert(t *testing.T) {
	state := State{}

	if selected, _ := state.ToggleSelection(); selected {
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

	toggled, _ := state.ToggleSelectionAndAdvance(5)
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

	toggled, _ := state.ToggleSelectionAndAdvance(5)
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
	testutil.WriteFile(t, filepath.Join(dir, "a.txt"))
	testutil.WriteFile(t, filepath.Join(dir, "b.txt"))

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
	testutil.WriteFile(t, filepath.Join(root, "z.txt"))

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
	if filepath.Base(state.Path.String()) != "sub" {
		t.Fatalf("Path = %q, want sub directory", state.Path)
	}

	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}
	if state.Path.String() != root {
		t.Fatalf("Path = %q, want %q", state.Path, root)
	}

	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "sub" {
		t.Fatalf("selected entry = %q ok=%v, want sub", entry.Name, ok)
	}
	subPath := filepath.Join(root, "sub")
	if len(state.History) < 2 || cleanPathString(state.History[0]) != cleanPathString(root) || cleanPathString(state.History[1]) != cleanPathString(subPath) {
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
	if state.Path.String() != root {
		t.Fatalf("Path = %q, want root %q", state.Path, root)
	}

	moved, err := state.HistoryBackward(5)
	if err != nil || !moved {
		t.Fatalf("HistoryBackward to sub: moved=%v err=%v", moved, err)
	}
	if state.Path.String() != sub {
		t.Fatalf("Path = %q, want sub %q", state.Path, sub)
	}

	moved, err = state.HistoryBackward(5)
	if err != nil || !moved {
		t.Fatalf("HistoryBackward to nested: moved=%v err=%v", moved, err)
	}
	if state.Path.String() != nested {
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

func TestHistoryNavigationRestoresPriorHighlightedEntry(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir(sub) error = %v", err)
	}
	testutil.WriteFile(t, filepath.Join(sub, "aaa.txt"))
	testutil.WriteFile(t, filepath.Join(sub, "zzz.txt"))

	state, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	entered, err := state.Enter(5)
	if err != nil || !entered {
		t.Fatalf("Enter(sub): entered=%v err=%v", entered, err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "zzz.txt" {
			state.Cursor = i
			break
		}
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "zzz.txt" {
		t.Fatalf("cursor on sub before leave = %q ok=%v, want zzz.txt", entry.Name, ok)
	}

	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}

	moved, err := state.HistoryBackward(5)
	if err != nil || !moved {
		t.Fatalf("HistoryBackward: moved=%v err=%v", moved, err)
	}
	entry, ok = state.CurrentEntry()
	if !ok || entry.Name != "zzz.txt" {
		t.Fatalf("after HistoryBackward entry = %q ok=%v, want zzz.txt", entry.Name, ok)
	}
	if state.Cursor == 0 {
		t.Fatal("cursor reset to row 0 after HistoryBackward, want prior highlight")
	}

	moved, err = state.HistoryForward(5)
	if err != nil || !moved {
		t.Fatalf("HistoryForward: moved=%v err=%v", moved, err)
	}
	entry, ok = state.CurrentEntry()
	if !ok || entry.Name != "sub" {
		t.Fatalf("after HistoryForward entry = %q ok=%v, want sub", entry.Name, ok)
	}
}

func TestNavigateToReenteredDirectoryRestoresHighlight(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir(sub) error = %v", err)
	}
	testutil.WriteFile(t, filepath.Join(sub, "keep.txt"))
	testutil.WriteFile(t, filepath.Join(sub, "target.txt"))

	state, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(5); err != nil {
		t.Fatalf("Enter(sub) error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "target.txt" {
			state.Cursor = i
			break
		}
	}
	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(5); err != nil {
		t.Fatalf("re-Enter(sub) error = %v", err)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "target.txt" {
		t.Fatalf("re-entered sub highlight = %q ok=%v, want target.txt", entry.Name, ok)
	}
}

func TestReenterDirectoryCentersRecalledCursor(t *testing.T) {
	const viewportRows = 5
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir(sub) error = %v", err)
	}
	for i := 0; i < 20; i++ {
		name := strconv.Itoa(i) + ".txt"
		testutil.WriteFile(t, filepath.Join(sub, name))
	}
	target := "10.txt"

	state, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(viewportRows); err != nil {
		t.Fatalf("Enter(sub) error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == target {
			state.Cursor = i
			break
		}
	}
	if err := state.Parent(viewportRows); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(viewportRows); err != nil {
		t.Fatalf("re-Enter(sub) error = %v", err)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != target {
		t.Fatalf("re-entered highlight = %q ok=%v, want %s", entry.Name, ok, target)
	}
	row := state.Cursor - state.ScrollOffset
	if row != viewportRows/2 {
		t.Fatalf("cursor viewport row = %d, want %d (centered)", row, viewportRows/2)
	}
}

func TestReenterDirectoryPinsTailWhenCenteringImpossible(t *testing.T) {
	const viewportRows = 5
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir(sub) error = %v", err)
	}
	for i := 0; i < 20; i++ {
		name := strconv.Itoa(i) + ".txt"
		testutil.WriteFile(t, filepath.Join(sub, name))
	}
	target := "9.txt"

	state, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(viewportRows); err != nil {
		t.Fatalf("Enter(sub) error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == target {
			state.Cursor = i
			break
		}
	}
	if err := state.Parent(viewportRows); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(viewportRows); err != nil {
		t.Fatalf("re-Enter(sub) error = %v", err)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != target {
		t.Fatalf("re-entered highlight = %q ok=%v, want %s", entry.Name, ok, target)
	}
	wantScroll := state.VisibleEntryCount() - viewportRows
	if state.ScrollOffset != wantScroll {
		t.Fatalf("ScrollOffset = %d, want %d (tail pinned)", state.ScrollOffset, wantScroll)
	}
	row := state.Cursor - state.ScrollOffset
	if row != viewportRows-1 {
		t.Fatalf("cursor viewport row = %d, want %d (last row)", row, viewportRows-1)
	}
}

func TestApplyListingScrollUsesFileListViewportRowsCallback(t *testing.T) {
	const liveVR = 19
	const staleVR = 5
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	state.FileListViewportRows = func() int { return liveVR }
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(staleVR); err != nil {
		t.Fatal(err)
	}
	if err := state.Parent(staleVR); err != nil {
		t.Fatal(err)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "sub" {
		t.Fatalf("highlight = %q ok=%v, want sub", entry.Name, ok)
	}
	row := state.Cursor - state.ScrollOffset
	mid := liveVR / 2
	if row != mid && row != liveVR-1 {
		t.Fatalf("viewport row = %d, want %d or %d; cursor=%d scroll=%d",
			row, mid, liveVR-1, state.Cursor, state.ScrollOffset)
	}
}

func TestParentCentersExitedDirectoryInListing(t *testing.T) {
	const viewportRows = 5
	root := t.TempDir()
	bar := filepath.Join(root, "bar")
	asdf := filepath.Join(bar, "asdf")
	if err := os.MkdirAll(asdf, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(bar, name), 0o755); err != nil {
			t.Fatalf("Mkdir(%s): %v", name, err)
		}
	}

	state, err := New(bar)
	if err != nil {
		t.Fatalf("New(bar): %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "asdf" {
			state.Cursor = i
			if _, err := state.Enter(viewportRows); err != nil {
				t.Fatalf("Enter(asdf): %v", err)
			}
			break
		}
	}
	if state.PathString() != asdf {
		t.Fatalf("path = %q, want %q", state.PathString(), asdf)
	}
	if err := state.Parent(viewportRows); err != nil {
		t.Fatalf("Parent(): %v", err)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "asdf" {
		t.Fatalf("highlight = %q ok=%v, want asdf", entry.Name, ok)
	}
	row := state.Cursor - state.ScrollOffset
	wantMid := viewportRows / 2
	if row != wantMid && row != viewportRows-1 {
		t.Fatalf("cursor viewport row = %d, want %d (centered) or %d (tail)", row, wantMid, viewportRows-1)
	}
}

func TestEnterSiblingRetainsPriorDescendantInHistory(t *testing.T) {
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
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "sub" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(5); err != nil {
		t.Fatalf("Enter(sub) error = %v", err)
	}
	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent() error = %v", err)
	}
	subPath := cleanPathString(sub)

	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "other" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(5); err != nil {
		t.Fatalf("Enter(other) error = %v", err)
	}
	if filepath.Clean(state.History[0]) != filepath.Clean(other) {
		t.Fatalf("History[0] = %q, want other dir %q", state.History[0], other)
	}
	foundSub := false
	for _, p := range state.History {
		if cleanPathString(p) == subPath {
			foundSub = true
			break
		}
	}
	if !foundSub {
		t.Fatalf("History = %v, want prior subdirectory %q retained", state.History, subPath)
	}
}

func TestParentThenSiblingRetainsPriorProjectDir(t *testing.T) {
	projects := t.TempDir()
	paras := filepath.Join(projects, "paras-commander")
	mc := filepath.Join(projects, "mc")
	for _, p := range []string{paras, mc} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatalf("Mkdir(%q) error = %v", p, err)
		}
	}

	state, err := New(paras)
	if err != nil {
		t.Fatalf("New(paras) error = %v", err)
	}
	if err := state.Parent(5); err != nil {
		t.Fatalf("Parent to projects error = %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "mc" {
			state.Cursor = i
			break
		}
	}
	if _, err := state.Enter(5); err != nil {
		t.Fatalf("Enter(mc) error = %v", err)
	}
	wantParas := cleanPathString(paras)
	for _, p := range state.History {
		if cleanPathString(p) == wantParas {
			return
		}
	}
	t.Fatalf("History = %v, want %q retained after projects -> mc", state.History, wantParas)
}

func TestEnterRegularFileIsInert(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "file.txt"))

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
	if state.Path.String() != dir {
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

func TestFilterHomeMovesCaretForPrefixInsert(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "notes.txt", Path: "/tmp/notes"},
		},
		Filter: FilterState{CaseInsensitive: true},
	}

	state.OpenFilter(5)
	for _, r := range "bc" {
		state.AppendFilterRune(r, 5)
	}
	state.MoveFilterCursorHome()
	state.AppendFilterRune('a', 5)

	if got := state.Filter.Query; got != "abc" {
		t.Fatalf("Filter.Query = %q, want %q", got, "abc")
	}
	if state.Filter.Cursor != 1 {
		t.Fatalf("Filter.Cursor = %d, want 1", state.Filter.Cursor)
	}

	state.MoveFilterCursorEnd()
	if state.Filter.Cursor != 3 {
		t.Fatalf("Filter.Cursor after MoveFilterCursorEnd = %d, want 3", state.Filter.Cursor)
	}

	state.MoveFilterCursorHome()
	state.BackspaceFilter(5)
	if got := state.Filter.Query; got != "abc" {
		t.Fatalf("BackspaceFilter at caret 0 should be a no-op, Query = %q, want %q", got, "abc")
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

// TestSortEntriesDirectMatchesApplySort confirms the extracted SortEntries free function (used
// directly by tree-mode child loads, since there's no *State to call ApplySort against a
// standalone []localfs.Entry) sorts identically to what ApplySort does through the receiver.
func TestSortEntriesDirectMatchesApplySort(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "z.txt", Path: "/tmp/z.txt"},
		{Name: "a.txt", Path: "/tmp/a.txt"},
		{Name: "m.txt", Path: "/tmp/m.txt"},
	}
	SortEntries(entries, SortState{Mode: SortName, Reverse: true}, nil, false)

	names := entryNames(entries)
	want := []string{"z.txt", "m.txt", "a.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

// TestSortEntriesUseDiskPrimaryFalseIgnoresDiskSorter confirms useDiskPrimary=false sorts by
// SortState.Mode even when a diskSorter is supplied and would rank differently — this is exactly
// how tree-mode child loads call SortEntries (see ApplyTreeChildLoad), forcing disk-primary sort
// off regardless of the panel's flat-mode disk-usage-idle-sort settings.
func TestSortEntriesUseDiskPrimaryFalseIgnoresDiskSorter(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "small.txt", Path: "/tmp/small.txt", Size: 1},
		{Name: "large.txt", Path: "/tmp/large.txt", Size: 100},
	}
	diskSorter := func(absPath string) (int64, bool) {
		// Disk totals disagree with SortSize (small.txt "big" on disk, large.txt "small").
		if absPath == "/tmp/small.txt" {
			return 9999, true
		}
		return 1, true
	}
	SortEntries(entries, SortState{Mode: SortSize}, diskSorter, false)

	names := entryNames(entries)
	want := []string{"small.txt", "large.txt"} // by Size ascending, diskSorter ignored
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q (useDiskPrimary=false must ignore diskSorter)", i, names[i], want[i])
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
		Path: pathloc.MustParse("/tmp"),
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

func TestClearSelectionClearsAllDirectories(t *testing.T) {
	state := State{
		Path: pathloc.MustParse("/tmp/here"),
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/here/a.txt"},
		},
		SelectedPaths: map[string]bool{
			"/tmp/here/a.txt":  true,
			"/tmp/other/b.txt": true,
		},
		SelectionsStripOrder: []string{"/tmp/other/b.txt"},
	}

	state.ClearSelection()
	if state.SelectedPaths != nil {
		t.Fatalf("SelectedPaths should be nil, got %#v", state.SelectedPaths)
	}
	if len(state.SelectionsStripOrder) != 0 {
		t.Fatalf("SelectionsStripOrder should be empty, got %v", state.SelectionsStripOrder)
	}
}

func TestSelectGroupByRegex(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "main.go", Path: "/tmp/main.go"},
			{Name: "main_test.go", Path: "/tmp/main_test.go"},
			{Name: "utils.go", Path: "/tmp/utils.go"},
			{Name: "README.md", Path: "/tmp/README.md"},
		},
	}

	if _, err := state.SelectGroup(`^main.*\.go$`, false, false, false, GroupPatternRegex, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("main.go should be selected")
	}
	if !state.IsSelected(state.Entries[1]) {
		t.Fatal("main_test.go should be selected")
	}
	if state.IsSelected(state.Entries[2]) {
		t.Fatal("utils.go should not be selected")
	}
	if state.IsSelected(state.Entries[3]) {
		t.Fatal("README.md should not be selected")
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

	if _, err := state.SelectGroup("*.go", false, false, false, GroupPatternShell, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

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

func TestSelectGroupByGlobCaseInsensitive(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "file.go", Path: "/tmp/file.go"},
			{Name: "FILE.go", Path: "/tmp/FILE.go"},
			{Name: "readme.md", Path: "/tmp/readme.md"},
		},
	}

	if _, err := state.SelectGroup("*.GO", false, false, false, GroupPatternShell, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("file.go should be selected (case-insensitive glob)")
	}
	if !state.IsSelected(state.Entries[1]) {
		t.Fatal("FILE.go should be selected (case-insensitive glob)")
	}
	if state.IsSelected(state.Entries[2]) {
		t.Fatal("readme.md should not be selected")
	}
}

func TestSelectGroupByGlobCaseSensitive(t *testing.T) {
	state := State{
		Entries: []localfs.Entry{
			{Name: "file.go", Path: "/tmp/file.go"},
			{Name: "file.GO", Path: "/tmp/file.GO"},
		},
	}

	if _, err := state.SelectGroup("*.go", false, false, true, GroupPatternShell, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

	if !state.IsSelected(state.Entries[0]) {
		t.Fatal("file.go should be selected")
	}
	if state.IsSelected(state.Entries[1]) {
		t.Fatal("file.GO should NOT be selected with case-sensitive glob")
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

	if _, err := state.UnselectGroup("*.go", false, false, false, GroupPatternShell, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

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

	if _, err := state.SelectGroup("*", true, false, false, GroupPatternShell, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

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
	if _, err := state.SelectGroup("main", false, false, true, GroupPatternSimple, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

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
	if _, err := state.SelectGroup("main", false, false, false, GroupPatternSimple, GroupSelectMeta{}); err != nil {
		t.Fatal(err)
	}

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

func TestCycleListingFormat(t *testing.T) {
	state := State{}
	state.CycleListingFormat()
	if state.ListFormat != ListFormatPerm {
		t.Fatalf("ListFormat = %v, want ListFormatPerm", state.ListFormat)
	}
	state.CycleListingFormat()
	if state.ListFormat != ListFormatBrief {
		t.Fatalf("ListFormat = %v, want ListFormatBrief", state.ListFormat)
	}
	state.CycleListingFormat()
	if state.ListFormat != ListFormatMtime {
		t.Fatalf("ListFormat = %v, want ListFormatMtime", state.ListFormat)
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
		DiskUsageIdleSortEligible: true, // mirrors DiskUsageShown after user-initiated analysis
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
	if err := state.load(pathloc.MustParse(dir), "", 10, noIndexCursorFallback, asyncLoadOpts{}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !state.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be true when listing is fully cached on load after analysis")
	}
	if len(state.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(state.Entries))
	}
	if state.Entries[0].Name != "zzz" {
		t.Fatalf("first entry = %q, want zzz (larger disk total sorts first)", state.Entries[0].Name)
	}
}

func TestLoadDoesNotApplyDiskTotalsSortWithoutAnalysisEligibility(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"aaa", "zzz"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
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
		// DiskUsageIdleSortEligible left false: selection-size cache must not flip sort.
	}
	state.DiskSorter = func(string) (int64, bool) { return 1, true }
	if err := state.load(pathloc.MustParse(dir), "", 10, noIndexCursorFallback, asyncLoadOpts{}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort must stay false without DiskUsageIdleSortEligible")
	}
	if state.Entries[0].Name != "aaa" {
		t.Fatalf("first entry = %q, want aaa (name sort)", state.Entries[0].Name)
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
		Path: pathloc.MustParse("/tmp"),
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

func TestSelectVisibleEntryInViewport(t *testing.T) {
	const viewportRows = 5
	entries := make([]localfs.Entry, 20)
	for i := range entries {
		name := fmt.Sprintf("%02d.dat", i)
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:         pathloc.MustParse("/tmp"),
		Entries:      entries,
		Sort:         SortState{Mode: SortName, DirectoriesFirst: false},
		Cursor:       0,
		ScrollOffset: 12,
	}
	state.ApplySort()
	if !state.SelectVisibleEntryInViewport("19.dat", viewportRows) {
		t.Fatal("SelectVisibleEntryInViewport(19.dat) = false, want true")
	}
	row := state.Cursor - state.ScrollOffset
	if row < 0 || row >= viewportRows {
		t.Fatalf("cursor row %d outside viewport, scroll=%d cursor=%d", row, state.ScrollOffset, state.Cursor)
	}
}

func TestSelectVisibleEntryCentered(t *testing.T) {
	entries := make([]localfs.Entry, 20)
	for i := range entries {
		name := fmt.Sprintf("%02d.txt", i)
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:         pathloc.MustParse("/tmp"),
		Entries:      entries,
		Sort:         SortState{Mode: SortName, DirectoriesFirst: false},
		ScrollOffset: 0,
	}
	state.ApplySort()
	if !state.SelectVisibleEntryCentered("10.txt", 5) {
		t.Fatal("SelectVisibleEntryCentered(10.txt) = false, want true")
	}
	if state.Cursor != 10 {
		t.Fatalf("Cursor = %d, want 10", state.Cursor)
	}
	if state.ScrollOffset != 8 {
		t.Fatalf("ScrollOffset = %d, want 8 (centered)", state.ScrollOffset)
	}
}

func TestEnsureCursorCentered(t *testing.T) {
	entries := make([]localfs.Entry, 20)
	for i := range entries {
		name := strconv.Itoa(i) + ".txt"
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:    pathloc.MustParse("/tmp"),
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

func TestMoveWithCenterScrolling(t *testing.T) {
	entries := make([]localfs.Entry, 20)
	for i := range entries {
		name := strconv.Itoa(i) + ".txt"
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:       pathloc.MustParse("/tmp"),
		Entries:    entries,
		Sort:       SortState{Mode: SortName, DirectoriesFirst: false},
		ScrollMode: ScrollModeCenter,
	}
	state.ApplySort()
	state.Cursor = 9
	state.Move(1, 5)
	if state.Cursor != 10 {
		t.Fatalf("Cursor = %d, want 10", state.Cursor)
	}
	if state.ScrollOffset != 8 {
		t.Fatalf("ScrollOffset = %d, want 8", state.ScrollOffset)
	}
}

func TestMoveWithoutCenterScrollingUsesMinimalScroll(t *testing.T) {
	entries := make([]localfs.Entry, 20)
	for i := range entries {
		name := strconv.Itoa(i) + ".txt"
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: entries,
		Sort:    SortState{Mode: SortName, DirectoriesFirst: false},
	}
	state.ApplySort()
	state.Cursor = 0
	state.ScrollOffset = 0
	state.Move(4, 5)
	if state.Cursor != 4 {
		t.Fatalf("Cursor = %d, want 4", state.Cursor)
	}
	if state.ScrollOffset != 0 {
		t.Fatalf("ScrollOffset = %d, want 0 (still visible without centering)", state.ScrollOffset)
	}
}

func TestEnsureCursorEdge(t *testing.T) {
	entries := make([]localfs.Entry, 40)
	for i := range entries {
		name := strconv.Itoa(i) + ".txt"
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:             pathloc.MustParse("/tmp"),
		Entries:          entries,
		Sort:             SortState{Mode: SortName, DirectoriesFirst: false},
		ScrollMode:       ScrollModeEdge,
		ScrollEdgeMargin: 5,
	}
	const viewportRows = 20
	state.ApplySort()
	state.Cursor = 5
	state.ScrollOffset = 0
	state.EnsureCursorEdge(viewportRows)
	if state.ScrollOffset != 0 {
		t.Fatalf("ScrollOffset = %d, want 0 inside edge margin", state.ScrollOffset)
	}
	state.Cursor = 15
	state.EnsureCursorEdge(viewportRows)
	if state.ScrollOffset != 1 {
		t.Fatalf("ScrollOffset = %d, want 1 after crossing bottom margin", state.ScrollOffset)
	}
}

func TestMoveWithEdgeScrolling(t *testing.T) {
	entries := make([]localfs.Entry, 40)
	for i := range entries {
		name := strconv.Itoa(i) + ".txt"
		entries[i] = localfs.Entry{Name: name, Path: filepath.Join("/tmp", name)}
	}
	state := State{
		Path:             pathloc.MustParse("/tmp"),
		Entries:          entries,
		Sort:             SortState{Mode: SortName, DirectoriesFirst: false},
		ScrollMode:       ScrollModeEdge,
		ScrollEdgeMargin: 5,
	}
	const viewportRows = 20
	state.ApplySort()
	state.Cursor = 5
	state.ScrollOffset = 0
	state.Move(9, viewportRows)
	if state.Cursor != 14 {
		t.Fatalf("Cursor = %d, want 14", state.Cursor)
	}
	if state.ScrollOffset != 0 {
		t.Fatalf("ScrollOffset = %d, want 0 (small moves stay put)", state.ScrollOffset)
	}
	state.Move(1, viewportRows)
	if state.Cursor != 15 {
		t.Fatalf("Cursor = %d, want 15", state.Cursor)
	}
	if state.ScrollOffset != 1 {
		t.Fatalf("ScrollOffset = %d, want 1 after crossing bottom margin", state.ScrollOffset)
	}
}

func TestRefreshDiskUsageOrderingPreservesCenteredScrollWhenNotRequested(t *testing.T) {
	const viewportRows = 5
	entries := make([]localfs.Entry, 15)
	sizes := map[string]int64{}
	for i := range entries {
		name := strconv.Itoa(i) + ".dat"
		p := filepath.Join("/tmp", name)
		entries[i] = localfs.Entry{Name: name, Path: p}
		sizes[p] = int64(i)
	}
	state := State{
		Path:               pathloc.MustParse("/tmp"),
		Entries:            entries,
		Sort:               SortState{Mode: SortName, DirectoriesFirst: false, DiskUsageIdleSizeSort: true},
		IdleDiskTotalsSort: true,
		Cursor:             7,
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		v, ok := sizes[filepath.Clean(abs)]
		return v, ok
	}
	state.ApplySort()
	state.applyHighlightScroll(viewportRows, true)
	if !state.cursorAppearsCentered(viewportRows) {
		t.Fatalf("precondition: scroll=%d cursor=%d not centered", state.ScrollOffset, state.Cursor)
	}
	// Reorder neighbors only so the highlighted entry keeps a middle index (regression: resort must not undo Parent-style centering).
	sizes[filepath.Clean(entries[6].Path)] = 1000
	sizes[filepath.Clean(entries[8].Path)] = 1
	state.RefreshDiskUsageOrdering(viewportRows, false)
	if !state.cursorAppearsCentered(viewportRows) {
		t.Fatalf("after disk reorder: scroll=%d cursor=%d, want centered viewport", state.ScrollOffset, state.Cursor)
	}
	ent, ok := state.CurrentEntry()
	if !ok || ent.Name != "7.dat" {
		name := ""
		if ok {
			name = ent.Name
		}
		t.Fatalf("cursor entry = %q ok=%v want 7.dat", name, ok)
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
		Path:               pathloc.MustParse("/tmp"),
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
		Path: pathloc.MustParse("/tmp"),
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
		Path: pathloc.MustParse("/tmp"),
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

func TestSelectedDirectoryPaths(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := State{
		SelectedPaths: map[string]bool{
			sub:  true,
			file: true,
		},
	}
	got := s.SelectedDirectoryPaths()
	if len(got) != 1 || got[0] != filepath.Clean(sub) {
		t.Fatalf("SelectedDirectoryPaths() = %v, want [%s]", got, filepath.Clean(sub))
	}
}

func TestPruneNestedPaths(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "a")
	child := filepath.Join(parent, "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(root, "c")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	got := PruneNestedPaths([]string{child, parent, sibling})
	want := []string{filepath.Clean(parent), filepath.Clean(sibling)}
	if len(got) != len(want) {
		t.Fatalf("PruneNestedPaths() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PruneNestedPaths() = %v, want %v", got, want)
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

func TestSelectionsStripDropsConflictRemovedDir(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cloverPath := filepath.Join(meadow, "clover.txt")
	testutil.WriteFile(t, cloverPath)

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Select meadow/, then enter it.
	if !state.SelectVisibleEntry("meadow") {
		t.Fatal("meadow not found")
	}
	if selected, _ := state.ToggleSelection(); !selected {
		t.Fatal("meadow should be selected")
	}
	entered, err := state.Enter(5)
	if err != nil || !entered {
		t.Fatalf("Enter meadow: err=%v entered=%v", err, entered)
	}
	// Selecting clover.txt conflicts with the selected ancestor dir; last selection wins.
	if !state.SelectVisibleEntry("clover.txt") {
		t.Fatal("clover.txt not found")
	}
	selected, conflictsRemoved := state.ToggleSelection()
	if !selected || !conflictsRemoved {
		t.Fatalf("ToggleSelection = (%v, %v), want selected with conflicts removed", selected, conflictsRemoved)
	}
	// Strip must not keep the removed dir; all selections are in the current dir, so it hides.
	if paths := state.SelectionsStripPaths(); len(paths) != 0 {
		t.Fatalf("SelectionsStripPaths = %v, want empty after conflict removal", paths)
	}
}

func TestCrossDirectorySelectionsAndStripOrder(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testutil.WriteFile(t, filepath.Join(root, "root.txt"))
	testutil.WriteFile(t, filepath.Join(sub, "sub.txt"))

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
	// Left sub: the visible strip lists ALL selections, including root.txt in the current dir.
	paths := state.SelectionsStripPaths()
	if len(paths) != 2 || !slices.Contains(paths, subTxtPath) || !slices.Contains(paths, rootEntryPath) {
		t.Fatalf("SelectionsStripPaths = %v, want sub.txt and root.txt", paths)
	}

	// Re-enter sub: strip still lists both selections.
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
	paths = state.SelectionsStripPaths()
	if len(paths) != 2 || !slices.Contains(paths, subTxtPath) || !slices.Contains(paths, rootEntryPath) {
		t.Fatalf("strip should list both selections while in sub, got %v", paths)
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

func TestNavigateFromRootPreservesPriorDirs(t *testing.T) {
	start := t.TempDir()
	state, err := New(start)
	if err != nil {
		t.Fatalf("New(%q): %v", start, err)
	}
	if err := state.NavigateTo("/", "", 10); err != nil {
		t.Fatalf("NavigateTo(/): %v", err)
	}
	if err := state.NavigateTo("/var", "", 10); err != nil {
		t.Fatalf("NavigateTo(/var): %v", err)
	}
	want := cleanPathString(start)
	for _, p := range state.History {
		if cleanPathString(p) == want {
			return
		}
	}
	t.Fatalf("history lost %q after / -> /var: %v", start, state.History)
}

func TestBestRecalledCursorPrefersNonemptyEntryName(t *testing.T) {
	dir := "/tmp/alpha"
	driver := &State{
		HistoryCursorByPath: map[string]historyCursorSnapshot{
			dir: {EntryName: "", Index: 0},
		},
	}
	follower := &State{
		HistoryCursorByPath: map[string]historyCursorSnapshot{
			dir: {EntryName: "b.txt", Index: 2},
		},
	}
	name, idx, ok := BestRecalledCursor(dir, driver, follower)
	if !ok || name != "b.txt" || idx != 2 {
		t.Fatalf("BestRecalledCursor = (%q, %d, %v), want (b.txt, 2, true)", name, idx, ok)
	}
}

func TestMergeHistoryCursorByPathKeepsNonemptySecondaryName(t *testing.T) {
	dir := "/tmp/alpha"
	merged := MergeHistoryCursorByPath(
		map[string]historyCursorSnapshot{dir: {EntryName: "b.txt", Index: 2}},
		map[string]historyCursorSnapshot{dir: {EntryName: "", Index: 0}},
	)
	snap, ok := merged[dir]
	if !ok || snap.EntryName != "b.txt" || snap.Index != 2 {
		t.Fatalf("merged[%q] = %+v ok=%v, want b.txt index 2", dir, snap, ok)
	}
}
