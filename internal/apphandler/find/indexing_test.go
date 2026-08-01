package find

import (
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const (
	findPollBatchLimit       = 8
	findPollBatchLimitLarge  = 1
	findPollBatchLimitMedium = 2
	findHiddenExpandPerTick  = 4
	findMaxConcurrentWalks   = 4
)

func findPollBatchLimitForCount(indexed int) int {
	switch {
	case indexed >= findIndexLargeThreshold:
		return findPollBatchLimitLarge
	case indexed >= findIndexMediumThreshold:
		return findPollBatchLimitMedium
	default:
		return findPollBatchLimit
	}
}

func findHiddenExpandPerTickForCount(indexed int) int {
	switch {
	case indexed >= findIndexLargeThreshold:
		return 1
	case indexed >= findIndexMediumThreshold:
		return 2
	default:
		return findHiddenExpandPerTick
	}
}

func findMaxConcurrentWalksForCount(indexed int) int {
	switch {
	case indexed >= findIndexLargeThreshold:
		return 1
	case indexed >= findIndexMediumThreshold:
		return 2
	default:
		return findMaxConcurrentWalks
	}
}

func TestAppendEmptyQueryDisplayIndices(t *testing.T) {
	t.Parallel()
	entries := []dialog.FindEntry{
		{RelLine: "a.txt"},
		{RelLine: "dir", IsDir: true},
	}
	got := appendEmptyQueryDisplayIndices(nil, entries, 0, false, false, 0)
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("all entries: got %v want [0 1]", got)
	}
	got = appendEmptyQueryDisplayIndices(nil, entries, 0, true, false, 0)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("only dirs: got %v want [1]", got)
	}
	got = appendEmptyQueryDisplayIndices([]int{0}, entries, 1, false, false, 2)
	if len(got) != 2 || got[1] != 1 {
		t.Fatalf("extend from 1: got %v want [0 1]", got)
	}
}

func TestFindIndexingSkipsRank(t *testing.T) {
	t.Parallel()
	st := &dialog.FindDialogState{Indexing: true, Query: ""}
	if !findIndexingSkipsRank(st) {
		t.Fatal("empty query during indexing should skip rank")
	}
	st.Query = "foo"
	if findIndexingSkipsRank(st) {
		t.Fatal("non-empty query during indexing should rank")
	}
	st.Indexing = false
	st.Query = ""
	if findIndexingSkipsRank(st) {
		t.Fatal("empty query after indexing should not skip rank via indexing guard")
	}
}

func TestFindPollBatchLimitForCount(t *testing.T) {
	t.Parallel()
	if got := findPollBatchLimitForCount(0); got != findPollBatchLimit {
		t.Fatalf("small index: got %d want %d", got, findPollBatchLimit)
	}
	if got := findPollBatchLimitForCount(findIndexMediumThreshold); got != findPollBatchLimitMedium {
		t.Fatalf("medium index: got %d want %d", got, findPollBatchLimitMedium)
	}
	if got := findPollBatchLimitForCount(findIndexLargeThreshold); got != findPollBatchLimitLarge {
		t.Fatalf("large index: got %d want %d", got, findPollBatchLimitLarge)
	}
}

func TestFindHiddenExpandPerTickForCount(t *testing.T) {
	t.Parallel()
	if got := findHiddenExpandPerTickForCount(0); got != findHiddenExpandPerTick {
		t.Fatalf("small index: got %d want %d", got, findHiddenExpandPerTick)
	}
	if got := findHiddenExpandPerTickForCount(findIndexMediumThreshold); got != 2 {
		t.Fatalf("medium index: got %d want 2", got)
	}
	if got := findHiddenExpandPerTickForCount(findIndexLargeThreshold); got != 1 {
		t.Fatalf("large index: got %d want 1", got)
	}
}

func TestFindIndexingCountThrottle(t *testing.T) {
	t.Parallel()
	if got := findIndexingCountThrottle(0); got != 500*time.Millisecond {
		t.Fatalf("small: got %v want 500ms", got)
	}
	if got := findIndexingCountThrottle(findIndexMediumThreshold); got != 750*time.Millisecond {
		t.Fatalf("medium: got %v want 750ms", got)
	}
	if got := findIndexingCountThrottle(findIndexLargeThreshold); got != 1000*time.Millisecond {
		t.Fatalf("large: got %v want 1000ms", got)
	}
}

func TestFindMaxConcurrentWalksForCount(t *testing.T) {
	t.Parallel()
	if got := findMaxConcurrentWalksForCount(0); got != findMaxConcurrentWalks {
		t.Fatalf("small: got %d want %d", got, findMaxConcurrentWalks)
	}
	if got := findMaxConcurrentWalksForCount(findIndexMediumThreshold); got != 2 {
		t.Fatalf("medium: got %d want 2", got)
	}
	if got := findMaxConcurrentWalksForCount(findIndexLargeThreshold); got != 1 {
		t.Fatalf("large: got %d want 1", got)
	}
}

func TestMaybeRenderFindIndexingThrottles(t *testing.T) {
	t.Parallel()
	h := &Handler{}
	st := &dialog.FindDialogState{Open: true, Indexing: true, IndexedCount: 100}
	if !h.maybeRenderFindIndexing(st) {
		t.Fatal("first render should pass")
	}
	if h.maybeRenderFindIndexing(st) {
		t.Fatal("immediate second render should be throttled")
	}
	h.lastIndexCountRenderAt = time.Now().Add(-600 * time.Millisecond)
	if !h.maybeRenderFindIndexing(st) {
		t.Fatal("render should pass after throttle interval")
	}
}

func TestEmptyQueryDisplayIndices(t *testing.T) {
	t.Parallel()
	isDirs := []bool{true, false, true, false}
	got := emptyQueryDisplayIndices(4, false, false, isDirs, 2)
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("cap all: got %v", got)
	}
	got = emptyQueryDisplayIndices(4, true, false, isDirs, 10)
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("only dirs: got %v", got)
	}
	got = emptyQueryDisplayIndices(3, true, false, []bool{true, false, true}, 10)
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("only dirs isDirs slice: got %v", got)
	}
}
