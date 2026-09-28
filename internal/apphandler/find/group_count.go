package find

import (
	"context"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// GroupCountPayload is posted when an async group-select match count (StartGroupCount)
// finishes. Gen ties it back to the request that started it; HandleGroupCount discards it once
// a newer request has superseded it. Watermark is how many Entries the count covered, so
// ExtendGroupCount knows where to resume counting newly indexed entries from.
type GroupCountPayload struct {
	Gen       uint64
	Files     int
	Dirs      int
	Watermark int
}

// groupCountAccum tracks the live group-select count's accumulated state between full
// StartGroupCount runs: the request and matcher that produced it, and how far (watermark) the
// accumulated files/dirs totals have counted into Entries. ExtendGroupCount advances it inline
// as new entries arrive from indexing; a full StartGroupCount replaces it outright.
type groupCountAccum struct {
	active   bool // a count is being tracked for the current pattern (even if it matched nothing)
	inFlight bool // an async full count is scheduled or running; ExtendGroupCount must wait

	req    GroupSelectRequest
	marked bool

	matcher    panel.GroupMatcher
	filesOnly  bool
	dirsOnly   bool
	fullPath   bool
	matchState bool // marks[path] == matchState is the "marked" filter (StartGroupCount's marked)

	files      int
	dirs       int
	watermark  int // entries covered so far by files/dirs
	entriesLen int // len(Entries) last observed, for IndexReplaced/shrink detection
}

// groupCountSnapshot is main-thread state copied out for the counting goroutine. Entries is
// append-only and RootPath is immutable for the life of a find session, so both are safe to
// read off-thread; MarkedPaths is a live, mutated map and is only safe to read because
// StopGroupCount fully waits for any in-flight count before a caller may mutate it.
type groupCountSnapshot struct {
	entries   []dialog.FindEntry
	indices   []int // nil means "all of entries[:len(entries)]" (unfiltered query)
	rootPath  string
	marks     map[string]bool
	filesOnly bool
	dirsOnly  bool
	fullPath  bool
	matched   bool // count entries whose marked state equals matched
	matcher   panel.GroupMatcher
}

// StartGroupCount (re)computes the group-select dialog's live match count off the main thread,
// debounced by config.UI.Find.QueryDebounceMS. Any previous in-flight or scheduled count is
// discarded first (StopGroupCount). Posts GroupCountPayload via the screen's event queue when
// the count finishes; stale/invalid patterns simply produce no payload.
func (h *Handler) StartGroupCount(req GroupSelectRequest, marked bool) {
	h.StopGroupCount()
	h.groupCountGen++
	gen := h.groupCountGen
	st := &h.model.FindDialog
	h.groupCountAcc = groupCountAccum{
		active:     req.Pattern != "",
		inFlight:   req.Pattern != "",
		req:        req,
		marked:     marked,
		entriesLen: len(st.Entries),
	}
	if req.Pattern == "" {
		return
	}
	matcher, err := newGroupMatcherForRequest(req.Pattern, req.PatternMode, req.CaseSensitive, req.FullPath)
	if err != nil {
		h.groupCountAcc.active = false
		h.groupCountAcc.inFlight = false
		return
	}
	h.groupCountAcc.matcher = matcher
	h.groupCountAcc.filesOnly = req.FilesOnly
	h.groupCountAcc.dirsOnly = req.DirsOnly
	h.groupCountAcc.fullPath = req.FullPath
	h.groupCountAcc.matchState = marked

	indices, covered := h.findResultIndicesCovered(st)
	snap := groupCountSnapshot{
		entries:   st.Entries,
		indices:   indices,
		rootPath:  st.RootPath,
		marks:     st.MarkedPaths,
		filesOnly: req.FilesOnly,
		dirsOnly:  req.DirsOnly,
		fullPath:  req.FullPath,
		matched:   marked,
		matcher:   matcher,
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.groupCountCancel = cancel
	delay := time.Duration(h.config.UI.Find.QueryDebounceMS) * time.Millisecond
	h.groupCount.Arm(delay, func() {
		// Commit to running (Add) only while still wanted, and do so under the same lock
		// StopGroupCount cancels under — so a cancel that happens first is guaranteed seen
		// here, and a commit that happens first is guaranteed waited for by StopGroupCount's
		// following Wait. This is what keeps Add/Wait race-free despite the debounce timer
		// and the cancellation racing on separate goroutines.
		h.groupCountMu.Lock()
		if ctx.Err() != nil {
			h.groupCountMu.Unlock()
			return
		}
		h.groupCountWG.Add(1)
		h.groupCountMu.Unlock()
		defer h.groupCountWG.Done()

		files, dirs, ok := countGroupMatchesSnapshot(ctx, snap)
		if !ok {
			return
		}
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(GroupCountPayload{Gen: gen, Files: files, Dirs: dirs, Watermark: covered}))
	})
}

// StopGroupCount cancels and fully waits for any in-flight or scheduled async count. Required
// before any MarkedPaths mutation (ApplyGroupSelect, dialog close) since a counting goroutine
// may still be reading that same map.
func (h *Handler) StopGroupCount() {
	h.groupCount.Invalidate()
	h.groupCountMu.Lock()
	cancel := h.groupCountCancel
	h.groupCountCancel = nil
	if cancel != nil {
		cancel()
	}
	h.groupCountMu.Unlock()
	h.groupCountWG.Wait()
	h.groupCountAcc.inFlight = false
}

// HandleGroupCount reports p's counts along with whether p is still current — Gen matches the
// latest StartGroupCount call. Callers should discard a stale (ok=false) payload. On a current
// payload, it also updates the accumulator so ExtendGroupCount can resume from p's watermark.
func (h *Handler) HandleGroupCount(p GroupCountPayload) (files, dirs int, ok bool) {
	ok = p.Gen == h.groupCountGen
	if ok {
		h.groupCountAcc.files = p.Files
		h.groupCountAcc.dirs = p.Dirs
		h.groupCountAcc.watermark = p.Watermark
		h.groupCountAcc.inFlight = false
		h.groupCountAcc.entriesLen = len(h.model.FindDialog.Entries)
	}
	return p.Files, p.Dirs, ok
}

// ExtendGroupCount advances the live group-select count for entries that arrived since the last
// full count (StartGroupCount) or extension, without a full async recount: it only visits
// entries beyond the watermark, so cost is proportional to the batch just indexed rather than
// the whole corpus. Called from the unchanged-key branch of updateGroupSelectPreview on every
// Run-loop iteration (including scan/rank interrupts) — a free hook for "corpus grew". changed
// is false when there was nothing to do: no active count, a full count is in-flight (it will
// catch up via its own watermark once its payload lands), or no new entries are covered yet.
// ponytail: runs inline on the main thread; move to a goroutine if a single batch ever gets huge.
func (h *Handler) ExtendGroupCount() (files, dirs int, changed bool) {
	acc := &h.groupCountAcc
	if !acc.active || acc.inFlight {
		return acc.files, acc.dirs, false
	}
	st := &h.model.FindDialog
	if len(st.Entries) < acc.entriesLen {
		// Entries were replaced wholesale (IndexReplaced / hidden-strip) or otherwise shrank:
		// the accumulated count and watermark no longer describe this corpus. Restart.
		h.StartGroupCount(acc.req, acc.marked)
		return acc.files, acc.dirs, true
	}
	acc.entriesLen = len(st.Entries)
	indices, covered := h.findResultIndicesCovered(st)
	if covered <= acc.watermark {
		return acc.files, acc.dirs, false
	}
	snap := groupCountSnapshot{
		entries:   st.Entries,
		indices:   deltaGroupCountIndices(indices, acc.watermark, covered),
		rootPath:  st.RootPath,
		marks:     st.MarkedPaths,
		filesOnly: acc.filesOnly,
		dirsOnly:  acc.dirsOnly,
		fullPath:  acc.fullPath,
		matched:   acc.matchState,
		matcher:   acc.matcher,
	}
	newFiles, newDirs, _ := countGroupMatchesSnapshot(context.Background(), snap)
	acc.files += newFiles
	acc.dirs += newDirs
	acc.watermark = covered
	return acc.files, acc.dirs, true
}

// deltaGroupCountIndices returns just the part of full needed to extend a count that has
// already covered watermark entries out of covered: entries at index >= watermark. full==nil
// means the query is empty (unfiltered, entries [0,covered) all count); the return is always
// non-nil since nil has the different meaning "no filter" to countGroupMatchesSnapshot.
func deltaGroupCountIndices(full []int, watermark, covered int) []int {
	if full == nil {
		if watermark >= covered {
			return []int{}
		}
		out := make([]int, 0, covered-watermark)
		for i := watermark; i < covered; i++ {
			out = append(out, i)
		}
		return out
	}
	out := make([]int, 0, len(full))
	for _, idx := range full {
		if idx >= watermark {
			out = append(out, idx)
		}
	}
	return out
}

// countGroupMatchesSnapshot is the pure match-count core over a snapshot, shared by the async
// count above and the synchronous CountGroupMatches (tests). ctx is checked every 4096 entries
// so a superseded count on a huge corpus aborts promptly instead of running to completion.
func countGroupMatchesSnapshot(ctx context.Context, snap groupCountSnapshot) (files, dirs int, ok bool) {
	n := len(snap.entries)
	if snap.indices != nil {
		n = len(snap.indices)
	}
	for i := 0; i < n; i++ {
		if i%4096 == 0 && ctx.Err() != nil {
			return 0, 0, false
		}
		idx := i
		if snap.indices != nil {
			idx = snap.indices[i]
		}
		if idx < 0 || idx >= len(snap.entries) {
			continue
		}
		ent := snap.entries[idx]
		if snap.filesOnly && ent.IsDir {
			continue
		}
		if snap.dirsOnly && !ent.IsDir {
			continue
		}
		path := filepath.Clean(ent.AbsPath(snap.rootPath))
		if path == "" {
			continue
		}
		name := path
		if !snap.fullPath {
			name = filepath.Base(path)
		}
		if !snap.matcher.Match(name) {
			continue
		}
		if snap.marks[path] != snap.matched {
			continue
		}
		if ent.IsDir {
			dirs++
		} else {
			files++
		}
	}
	return files, dirs, true
}
