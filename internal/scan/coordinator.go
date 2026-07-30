package scan

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/panel"
)

type walkHandle struct {
	root string
	sess *rootWalk
}

type coordinatorCmd struct {
	start           *StartOpts
	cancel          bool
	includeHidden   *bool
	restart         *StartOpts
	narrowSelection []string
	widen           bool
	match           *MatchRequest
	done            chan struct{}
}

type walkDoneMsg struct {
	gen          int
	root         string
	err          error
	skippedDirs  []string
	skippedFiles []Entry
}

type walkBatchMsg struct {
	gen   int
	batch []Entry
}

// Coordinator owns find index ingest, walks, hidden expansion, and background match.
type Coordinator struct {
	defaultWalk fswalk.Params
	notify      func(Event)

	cmd      chan coordinatorCmd
	internal chan any

	sessMu sync.RWMutex
	active *session
}

// NewCoordinator starts the coordinator goroutine. notify is called from the coordinator thread.
func NewCoordinator(walk fswalk.Params, notify func(Event)) *Coordinator {
	c := &Coordinator{
		defaultWalk: walk,
		notify:      notify,
		cmd:         make(chan coordinatorCmd, 8),
		internal:    make(chan any, 64),
	}
	go c.loop()
	return c
}

// Start begins indexing with opts.
func (c *Coordinator) Start(opts StartOpts) {
	c.cmd <- coordinatorCmd{start: &opts}
}

// Cancel stops the current session.
func (c *Coordinator) Cancel() {
	c.cmd <- coordinatorCmd{cancel: true}
}

// SetIncludeHidden toggles hidden indexing policy. Turning hidden off strips asynchronously.
func (c *Coordinator) SetIncludeHidden(on bool) {
	if on {
		done := make(chan struct{})
		c.cmd <- coordinatorCmd{includeHidden: &on, done: done}
		<-done
		return
	}
	c.cmd <- coordinatorCmd{includeHidden: &on}
}

// Restart clears the index and starts fresh (stay-on-volume toggle).
func (c *Coordinator) Restart(opts StartOpts) {
	done := make(chan struct{})
	c.cmd <- coordinatorCmd{restart: &opts, done: done}
	<-done
}

// NarrowSelection stops walks outside scope and filters the index.
func (c *Coordinator) NarrowSelection(roots []string) {
	out := make([]string, len(roots))
	copy(out, roots)
	done := make(chan struct{})
	c.cmd <- coordinatorCmd{narrowSelection: out, done: done}
	<-done
}

// WidenSelection starts the panel root walk if missing.
func (c *Coordinator) Widen() {
	c.cmd <- coordinatorCmd{widen: true}
}

// RequestMatch schedules a background rank pass.
func (c *Coordinator) RequestMatch(req MatchRequest) {
	r := req
	c.cmd <- coordinatorCmd{match: &r}
}

// Snapshot returns a copy of the active session index (nil when idle).
func (c *Coordinator) Snapshot() []Entry {
	c.sessMu.RLock()
	s := c.active
	c.sessMu.RUnlock()
	if s == nil {
		return nil
	}
	return s.idx.Snapshot()
}

// IndexedCount returns the live indexed entry count.
func (c *Coordinator) IndexedCount() int {
	c.sessMu.RLock()
	s := c.active
	c.sessMu.RUnlock()
	if s == nil {
		return 0
	}
	return s.idx.Len()
}

type session struct {
	gen    int
	ctx    context.Context
	cancel context.CancelFunc

	opts StartOpts
	idx  *Index

	walks          map[string]*walkHandle
	completedRoots map[string]struct{}

	hidden hiddenState

	matchGen     int
	matchCancel  context.CancelFunc
	matchRunning bool

	lastErr error

	coord *Coordinator
}

func (c *Coordinator) loop() {
	for {
		select {
		case cmd := <-c.cmd:
			switch {
			case cmd.cancel:
				c.setActive(nil)
			case cmd.start != nil:
				c.setActive(c.newSession(*cmd.start))
				c.active.startRoots(false)
			case cmd.restart != nil:
				c.setActive(c.newSession(*cmd.restart))
				c.active.startRoots(false)
			case cmd.includeHidden != nil:
				if s := c.activeSession(); s != nil {
					s.setIncludeHidden(*cmd.includeHidden)
				}
			case cmd.narrowSelection != nil:
				if s := c.activeSession(); s != nil {
					s.narrowSelection(cmd.narrowSelection)
				}
			case cmd.widen:
				if s := c.activeSession(); s != nil {
					s.widen()
				}
			case cmd.match != nil:
				if s := c.activeSession(); s != nil {
					s.scheduleMatch(*cmd.match)
				}
			}
			if cmd.done != nil {
				close(cmd.done)
			}
		case msg := <-c.internal:
			switch m := msg.(type) {
			case walkBatchMsg:
				if s := c.activeSession(); s != nil && m.gen == s.gen {
					s.ingestBatch(m.batch)
				}
			case walkDoneMsg:
				if s := c.activeSession(); s != nil && m.gen == s.gen {
					s.onWalkDone(m)
				}
			case matchDoneMsg:
				if s := c.activeSession(); s != nil && m.gen == s.matchGen {
					s.matchRunning = false
					s.emit(Event{MatchResult: true, Match: m.out})
				}
			case stripDoneMsg:
				if s := c.activeSession(); s != nil && m.gen == s.gen {
					s.applyStripDone(m.entries)
				}
			}
		}
	}
}

func (c *Coordinator) setActive(s *session) {
	c.sessMu.Lock()
	old := c.active
	c.active = s
	c.sessMu.Unlock()
	if old != nil {
		go old.stop()
	}
}

func (c *Coordinator) activeSession() *session {
	c.sessMu.RLock()
	defer c.sessMu.RUnlock()
	return c.active
}

func (c *Coordinator) newSession(opts StartOpts) *session {
	if opts.Walk.InitialWorkers == 0 {
		opts.Walk = c.defaultWalk
	}
	gen := opts.Gen
	if gen == 0 {
		gen = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{
		gen:            gen,
		ctx:            ctx,
		cancel:         cancel,
		opts:           opts,
		idx:            newIndex(),
		walks:          make(map[string]*walkHandle),
		completedRoots: make(map[string]struct{}),
		coord:          c,
	}
	s.hidden = *newHiddenState()
	return s
}

func (s *session) stop() {
	s.cancelPendingMatch()
	for _, w := range s.walks {
		if w.sess != nil {
			w.sess.Close()
		}
	}
	s.cancel()
	s.walks = nil
}

func (s *session) emit(ev Event) {
	ev.Gen = s.gen
	if s.coord.notify != nil {
		s.coord.notify(ev)
	}
}

func (s *session) emitCount() {
	s.emit(Event{
		CountUpdate: true,
		Count:       s.idx.Len(),
		WalkWorkers: s.maxWalkWorkers(),
		IndexActive: len(s.walks) > 0 || s.hiddenWorkPending(),
	})
}

func (s *session) maxWalkWorkers() int {
	max := 0
	for _, w := range s.walks {
		if w.sess != nil {
			if n := w.sess.Workers(); n > max {
				max = n
			}
		}
	}
	return max
}

func (s *session) scopeRoots() []string {
	if s.opts.SearchOnlySelections && len(s.opts.SelectionRoots) > 0 {
		out := make([]string, len(s.opts.SelectionRoots))
		for i, r := range s.opts.SelectionRoots {
			out[i] = filepath.Clean(r)
		}
		return out
	}
	return []string{filepath.Clean(s.opts.DisplayRoot)}
}

func (s *session) volumeSkip() diskusage.ShouldIgnoreFolder {
	return diskusage.ComposeListingVolumeIgnore(nil, s.opts.VolumeGate)
}

func (s *session) shouldSkipForRoot(root string, skipIndexedSelectionSubtrees bool) func(string) bool {
	volumeSkip := s.volumeSkip()
	if !skipIndexedSelectionSubtrees {
		if volumeSkip == nil {
			return nil
		}
		return volumeSkip
	}
	skipRoots := s.selectionSkipRoots()
	return func(abs string) bool {
		if volumeSkip != nil && volumeSkip(abs) {
			return true
		}
		abs = filepath.Clean(abs)
		for _, sr := range skipRoots {
			if panel.IsStrictPathDescendant(sr, abs) {
				return true
			}
		}
		return false
	}
}

func (s *session) selectionSkipRoots() []string {
	if !s.opts.SearchOnlySelections {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for root := range s.walks {
		r := filepath.Clean(root)
		if r == filepath.Clean(s.opts.DisplayRoot) {
			continue
		}
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	for root := range s.completedRoots {
		r := filepath.Clean(root)
		if r == filepath.Clean(s.opts.DisplayRoot) {
			continue
		}
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	for _, r := range s.opts.SelectionRoots {
		r = filepath.Clean(r)
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

func (s *session) startRoots(skipIndexedSelectionSubtrees bool) {
	for _, root := range s.scopeRoots() {
		s.startWalk(root, skipIndexedSelectionSubtrees)
	}
	s.emitCount()
	s.maybeFinish()
}

func (s *session) startWalk(root string, skipIndexedSelectionSubtrees bool) {
	root = filepath.Clean(root)
	if root == "" {
		return
	}
	if _, exists := s.walks[root]; exists {
		return
	}
	if _, done := s.completedRoots[root]; done {
		return
	}

	gen := s.gen
	wopts := WalkOptions{
		Root:          root,
		IncludeHidden: s.opts.IncludeHidden,
		Gitignore:     s.opts.Gitignore,
		ShouldSkipDir: s.shouldSkipForRoot(root, skipIndexedSelectionSubtrees),
	}
	sess := startRootWalk(s.ctx, root, wopts, s.opts.Walk)
	s.walks[root] = &walkHandle{root: root, sess: sess}
	go s.readWalk(sess, gen, root)
}

func (s *session) readWalk(sess *rootWalk, gen int, root string) {
	for batch := range sess.Results() {
		select {
		case s.coord.internal <- walkBatchMsg{gen: gen, batch: batch}:
		case <-s.ctx.Done():
			return
		}
	}
	<-sess.Done()
	msg := walkDoneMsg{
		gen:          gen,
		root:         root,
		err:          sess.Err(),
		skippedDirs:  sess.SkippedHiddenDirs(),
		skippedFiles: sess.SkippedHiddenFiles(),
	}
	select {
	case s.coord.internal <- msg:
	case <-s.ctx.Done():
	}
}

func (s *session) ingestBatch(batch []Entry) {
	if len(batch) == 0 {
		return
	}
	if s.idx.Append(s.opts.DisplayRoot, batch) > 0 {
		s.emitCount()
	}
	s.maybeFinish()
}

func (s *session) onWalkDone(msg walkDoneMsg) {
	delete(s.walks, msg.root)
	s.completedRoots[msg.root] = struct{}{}
	if len(msg.skippedDirs) > 0 || len(msg.skippedFiles) > 0 {
		s.hidden.mergeSkipped(msg.skippedDirs, msg.skippedFiles)
		if s.opts.IncludeHidden {
			s.expandHidden()
		}
	}
	if msg.err != nil && s.ctx.Err() == nil {
		s.lastErr = msg.err
	}
	s.emitCount()
	s.maybeFinish()
}

func (s *session) allWalksDone() bool {
	return len(s.walks) == 0
}

func (s *session) hiddenWorkPending() bool {
	if !s.opts.IncludeHidden {
		return false
	}
	h := &s.hidden
	return len(h.expandPending) > 0 ||
		h.expandNext < len(h.pendingDirs) ||
		h.filesSpliceAt < len(h.pendingFiles)
}

func (s *session) maybeFinish() {
	if !s.allWalksDone() {
		return
	}
	for s.opts.IncludeHidden && s.processHidden() {
	}
	if s.hiddenWorkPending() {
		return
	}
	errStr := ""
	if s.lastErr != nil {
		errStr = s.lastErr.Error()
	}
	s.emit(Event{IndexFinished: true, Count: s.idx.Len(), WalkWorkers: 0, IndexErr: errStr})
}

func (s *session) setIncludeHidden(on bool) {
	s.opts.IncludeHidden = on
	if on {
		s.expandHidden()
		s.emitCount()
		s.maybeFinish()
		return
	}
	s.stopExpandedHiddenWalks()
	s.hidden.expandPending = nil
	gen := s.gen
	displayRoot := s.opts.DisplayRoot
	snap := s.idx.Snapshot()
	go func() {
		filtered := stripHiddenEntriesByName(snap, displayRoot)
		s.coord.internal <- stripDoneMsg{gen: gen, entries: filtered}
	}()
}

func (s *session) applyStripDone(filtered []Entry) {
	s.idx.ReplaceEntries(s.opts.DisplayRoot, filtered)
	s.hidden.expandedRoots = nil
	s.hidden.filesSpliceAt = 0
	s.hidden.expandNext = 0
	s.emit(Event{
		CountUpdate:   true,
		Count:         s.idx.Len(),
		WalkWorkers:   s.maxWalkWorkers(),
		IndexActive:   len(s.walks) > 0 || s.hiddenWorkPending(),
		IndexReplaced: true,
	})
	s.maybeFinish()
}

func (s *session) expandHidden() {
	if !s.opts.IncludeHidden {
		return
	}
	if batch := s.hidden.spliceFilesBatch(); len(batch) > 0 {
		s.ingestBatch(batch)
	}
	s.hidden.enqueueDirs(hiddenEnqueuePerTick)
	s.processHidden()
}

func (s *session) processHidden() bool {
	if !s.opts.IncludeHidden {
		return false
	}
	if batch := s.hidden.spliceFilesBatch(); len(batch) > 0 {
		s.ingestBatch(batch)
	}
	s.hidden.enqueueDirs(hiddenEnqueuePerTick)
	limit := hiddenExpandPerTickForCount(s.idx.Len())
	maxWalks := maxConcurrentWalksForCount(s.idx.Len())
	started := false
	for limit > 0 && len(s.hidden.expandPending) > 0 {
		if len(s.walks) >= maxWalks {
			return true
		}
		dir := s.hidden.expandPending[0]
		s.hidden.expandPending = s.hidden.expandPending[1:]
		limit--
		started = true
		s.ingestBatch([]Entry{dirEntryForHiddenDir(dir)})
		delete(s.completedRoots, filepath.Clean(dir))
		s.startWalk(dir, false)
	}
	if len(s.hidden.expandPending) > 0 ||
		s.hidden.expandNext < len(s.hidden.pendingDirs) ||
		s.hidden.filesSpliceAt < len(s.hidden.pendingFiles) {
		if batch := s.hidden.spliceFilesBatch(); len(batch) > 0 {
			s.ingestBatch(batch)
		}
		s.hidden.enqueueDirs(hiddenEnqueuePerTick)
		return true
	}
	return started
}

func (s *session) stopExpandedHiddenWalks() {
	if len(s.hidden.expandedRoots) == 0 {
		return
	}
	for root, w := range s.walks {
		if _, ok := s.hidden.expandedRoots[root]; !ok {
			continue
		}
		if w.sess != nil {
			w.sess.Abort()
		}
		delete(s.walks, root)
		delete(s.completedRoots, root)
	}
}

func (s *session) narrowSelection(roots []string) {
	s.opts.SelectionRoots = roots
	s.opts.SearchOnlySelections = true
	panelRoot := filepath.Clean(s.opts.DisplayRoot)
	allowed := make(map[string]struct{}, len(roots))
	for _, r := range roots {
		allowed[filepath.Clean(r)] = struct{}{}
	}
	for root, w := range s.walks {
		root = filepath.Clean(root)
		if root == panelRoot {
			continue
		}
		if _, ok := allowed[root]; !ok {
			if w.sess != nil {
				w.sess.Close()
			}
			delete(s.walks, root)
			delete(s.completedRoots, root)
		}
	}
	var filtered []Entry
	s.idx.View(func(entries []Entry, _ int) {
		filtered = filterEntriesToScope(entries, s.opts.DisplayRoot, roots)
	})
	s.idx.ReplaceEntries(s.opts.DisplayRoot, filtered)
	s.emitCount()
	s.maybeFinish()
}

func (s *session) widen() {
	panelRoot := filepath.Clean(s.opts.DisplayRoot)
	s.opts.SearchOnlySelections = false
	if _, active := s.walks[panelRoot]; active {
		s.emitCount()
		s.maybeFinish()
		return
	}
	if _, done := s.completedRoots[panelRoot]; done {
		s.emitCount()
		s.maybeFinish()
		return
	}
	s.startWalk(panelRoot, true)
	s.emitCount()
	s.maybeFinish()
}

func (s *session) cancelPendingMatch() {
	s.matchGen++
	if s.matchCancel != nil {
		s.matchCancel()
		s.matchCancel = nil
	}
	s.matchRunning = false
}

func (s *session) scheduleMatch(req MatchRequest) {
	s.cancelPendingMatch()
	s.matchGen = req.Gen
	ctx, cancel := context.WithCancel(s.ctx)
	s.matchCancel = cancel
	s.matchRunning = true
	gen := req.Gen
	go func() {
		lines, isDirs := s.idx.LinesAndDirs()
		out := runMatch(lines, isDirs, req, func() bool {
			select {
			case <-ctx.Done():
				return true
			default:
				return false
			}
		})
		if ctx.Err() != nil {
			return
		}
		s.coord.internal <- matchDoneMsg{gen: gen, out: out}
	}()
}

type matchDoneMsg struct {
	gen int
	out MatchOutput
}

type stripDoneMsg struct {
	gen     int
	entries []Entry
}
