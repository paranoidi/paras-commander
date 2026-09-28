package find

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestMatchingFindPathsFullPathMatchesDirComponent(t *testing.T) {
	t.Parallel()
	st := &dialog.FindDialogState{
		RootPath: "/root",
		Entries: []dialog.FindEntry{
			{RelLine: "src/main.go"},
			{RelLine: "lib/main.go"},
		},
		IndexedCount: 2,
	}

	basenameMatcher, err := panel.NewGroupMatcher("*src*", panel.GroupPatternShell, false)
	if err != nil {
		t.Fatal(err)
	}
	if paths, _ := matchingFindPaths(st, nil, false, false, false, basenameMatcher); len(paths) != 0 {
		t.Fatalf("basename match should not see the dir component, got %v", paths)
	}

	fullPathMatcher, err := panel.NewGroupMatcherFullPath("*src*", panel.GroupPatternShell, false)
	if err != nil {
		t.Fatal(err)
	}
	paths, isDir := matchingFindPaths(st, nil, false, false, true, fullPathMatcher)
	want := filepath.Clean("/root/src/main.go")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("full-path match = %v, want [%s]", paths, want)
	}
	if isDir[want] {
		t.Fatalf("main.go misreported as a dir")
	}
}

func newTestScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 20)
	return screen
}

// drainGroupCountPayloads polls screen for up to timeout, collecting every posted
// GroupCountPayload, then returns once nothing has arrived for a full idle window.
func drainGroupCountPayloads(screen tcell.SimulationScreen, timeout time.Duration) []GroupCountPayload {
	var out []GroupCountPayload
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !screen.HasPendingEvent() {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		ev := screen.PollEvent()
		if ie, ok := ev.(*tcell.EventInterrupt); ok {
			if p, ok := ie.Data().(GroupCountPayload); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

func TestStartGroupCountOnlyLastGenDelivered(t *testing.T) {
	screen := newTestScreen(t)
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:     true,
			RootPath: "/root",
			Entries: []dialog.FindEntry{
				{RelLine: "one.txt"},
				{RelLine: "two.txt"},
				{RelLine: "three.md"},
			},
			IndexedCount: 3,
		},
	}
	h := newTestFindHandler(host, model)
	h.screen = screen
	h.config.UI.Find.QueryDebounceMS = 30

	// Two requests fired back-to-back, well within the debounce delay: the first must never
	// fire (StopGroupCount inside the second StartGroupCount invalidates it before its timer
	// expires), so only the second's count should ever be posted.
	h.StartGroupCount(GroupSelectRequest{Pattern: "*.md", PatternMode: panel.GroupPatternShell}, false)
	h.StartGroupCount(GroupSelectRequest{Pattern: "*.txt", PatternMode: panel.GroupPatternShell}, false)
	wantGen := h.groupCountGen

	payloads := drainGroupCountPayloads(screen, 500*time.Millisecond)
	if len(payloads) != 1 {
		t.Fatalf("payloads = %+v, want exactly 1", payloads)
	}
	if payloads[0].Gen != wantGen {
		t.Fatalf("payload gen = %d, want %d", payloads[0].Gen, wantGen)
	}
	if payloads[0].Files != 2 {
		t.Fatalf("payload files = %d, want 2 (one.txt, two.txt)", payloads[0].Files)
	}
}

func TestHandleGroupCountDiscardsStaleGen(t *testing.T) {
	t.Parallel()
	host := &fakeFindHost{}
	model := &ui.Model{FindDialog: dialog.FindDialogState{Open: true}}
	h := newTestFindHandler(host, model)
	h.groupCountGen = 5

	if _, _, ok := h.HandleGroupCount(GroupCountPayload{Gen: 4}); ok {
		t.Fatal("stale gen should be discarded")
	}
	files, dirs, ok := h.HandleGroupCount(GroupCountPayload{Gen: 5, Files: 3, Dirs: 1})
	if !ok || files != 3 || dirs != 1 {
		t.Fatalf("current gen: got files=%d dirs=%d ok=%v, want 3 1 true", files, dirs, ok)
	}
}

// TestStopGroupCountThenMutateMarksIsRaceFree exercises the exact ordering ApplyGroupSelect and
// closeGroupSelect rely on: StopGroupCount must fully wait for an in-flight count (which reads
// MarkedPaths) before the caller mutates that same map. Run under -race: a broken Add/Done
// pairing that lets StopGroupCount return early would show up here as a concurrent map
// read/write.
func TestStopGroupCountThenMutateMarksIsRaceFree(t *testing.T) {
	screen := newTestScreen(t)
	entries := make([]dialog.FindEntry, 20000)
	marks := make(map[string]bool, len(entries))
	for i := range entries {
		entries[i] = dialog.FindEntry{RelLine: fmt.Sprintf("file_%05d.txt", i)}
	}
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:         true,
			RootPath:     "/root",
			Entries:      entries,
			IndexedCount: len(entries),
			MarkedPaths:  marks,
		},
	}
	h := newTestFindHandler(host, model)
	h.screen = screen
	h.config.UI.Find.QueryDebounceMS = 1

	h.StartGroupCount(GroupSelectRequest{Pattern: "*.txt", PatternMode: panel.GroupPatternShell}, false)
	h.StopGroupCount()

	st := &h.model.FindDialog
	st.MarkedPaths[filepath.Join("/root", "file_00000.txt")] = true
	delete(st.MarkedPaths, filepath.Join("/root", "file_00001.txt"))
}

// TestExtendGroupCountAddsOnlyNewMatches verifies the watermark + incremental-delta path:
// once a full count has landed, appending more entries (simulating indexing progress) and
// calling ExtendGroupCount adds only the newly covered matches instead of recounting everything
// or staying stale until the next full StartGroupCount.
func TestExtendGroupCountAddsOnlyNewMatches(t *testing.T) {
	screen := newTestScreen(t)
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:         true,
			RootPath:     "/root",
			Entries:      makeFindEntries(5),
			IndexedCount: 5,
		},
	}
	h := newTestFindHandler(host, model)
	h.screen = screen
	h.config.UI.Find.QueryDebounceMS = 5

	h.StartGroupCount(GroupSelectRequest{Pattern: "*.txt", PatternMode: panel.GroupPatternShell}, false)
	payloads := drainGroupCountPayloads(screen, 500*time.Millisecond)
	if len(payloads) != 1 {
		t.Fatalf("payloads = %+v, want 1", payloads)
	}
	files, dirs, ok := h.HandleGroupCount(payloads[0])
	if !ok || files != 5 || dirs != 0 {
		t.Fatalf("initial count = files=%d dirs=%d ok=%v, want 5 0 true", files, dirs, ok)
	}

	// Indexing continues: three more matching entries arrive.
	st := &h.model.FindDialog
	st.Entries = append(st.Entries, makeFindEntries(3)...)
	st.IndexedCount = len(st.Entries)

	files, dirs, changed := h.ExtendGroupCount()
	if !changed {
		t.Fatal("ExtendGroupCount should have counted the newly appended entries")
	}
	if files != 8 || dirs != 0 {
		t.Fatalf("extended count = files=%d dirs=%d, want 8 0", files, dirs)
	}

	if _, _, changed := h.ExtendGroupCount(); changed {
		t.Fatal("ExtendGroupCount should be a no-op once nothing new is covered")
	}
}

// TestFindResultIndicesCoveredUsesStaleFullRanked exercises the group-select-preview helper
// directly: with a non-empty query and a FullRanked cache that predates entries growing
// (findFullRankedCacheValid would reject it), findResultIndicesCovered must still report the
// stale FullRanked's coverage rather than 0 — that's what keeps the live count (and
// ApplyGroupSelect) non-zero while indexing is still running.
func TestFindResultIndicesCoveredUsesStaleFullRanked(t *testing.T) {
	t.Parallel()
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:                 true,
			Query:                "file",
			RootPath:             "/root",
			Entries:              makeFindEntries(8),
			FullRanked:           []int{0, 1, 2},
			FullRankedGen:        2,
			FullRankedEntriesLen: 3,
		},
	}
	h := newTestFindHandler(host, model)
	h.rankGen = 2
	st := &h.model.FindDialog

	indices, covered := h.findResultIndicesCovered(st)
	if covered != 3 {
		t.Fatalf("covered = %d, want 3 (stale FullRankedEntriesLen), not 0", covered)
	}
	if len(indices) != 3 {
		t.Fatalf("indices = %v, want the stale FullRanked", indices)
	}
}

// TestExtendGroupCountRestartsOnEntriesShrink verifies that when Entries is replaced wholesale
// with fewer entries (IndexReplaced, e.g. a hidden-strip toggle), ExtendGroupCount discards the
// now-invalid accumulated count and restarts a full async count instead of silently trusting a
// watermark that no longer describes the corpus.
func TestExtendGroupCountRestartsOnEntriesShrink(t *testing.T) {
	screen := newTestScreen(t)
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:         true,
			RootPath:     "/root",
			Entries:      makeFindEntries(5),
			IndexedCount: 5,
		},
	}
	h := newTestFindHandler(host, model)
	h.screen = screen
	h.config.UI.Find.QueryDebounceMS = 5

	h.StartGroupCount(GroupSelectRequest{Pattern: "*.txt", PatternMode: panel.GroupPatternShell}, false)
	payloads := drainGroupCountPayloads(screen, 500*time.Millisecond)
	if len(payloads) != 1 {
		t.Fatalf("payloads = %+v, want 1", payloads)
	}
	if _, _, ok := h.HandleGroupCount(payloads[0]); !ok {
		t.Fatal("expected initial payload to apply")
	}

	// IndexReplaced: Entries wholesale-replaced with fewer entries than the count covered.
	st := &h.model.FindDialog
	st.Entries = makeFindEntries(2)
	st.IndexedCount = 2

	if _, _, changed := h.ExtendGroupCount(); !changed {
		t.Fatal("shrink should trigger a restart and report changed")
	}
	if !h.groupCountAcc.inFlight {
		t.Fatal("restart should have armed a fresh async count")
	}

	payloads = drainGroupCountPayloads(screen, 500*time.Millisecond)
	if len(payloads) != 1 {
		t.Fatalf("restart payloads = %+v, want 1", payloads)
	}
	files, _, ok := h.HandleGroupCount(payloads[0])
	if !ok || files != 2 {
		t.Fatalf("restarted count = files=%d ok=%v, want 2 true", files, ok)
	}
}
