package compare

import (
	"bytes"
	"cmp"
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// dedupProgressInterval rate-limits dedup walk and hashing progress publishes.
// ponytail: fixed 500ms; promote to config if someone wants to tune it.
const dedupProgressInterval = 500 * time.Millisecond

// DedupPhase is the dedup session lifecycle stage.
type DedupPhase int

const (
	DedupWalking DedupPhase = iota
	DedupAwaitConfirm
	DedupHashing
	DedupDone
	DedupError
	DedupCanceled
)

// DedupFile is one member of a duplicate group.
type DedupFile struct {
	Rel string
	Abs pathloc.Path
}

// DedupGroup is a set of files with identical content (same size + SHA-256).
type DedupGroup struct {
	Hash  [32]byte
	Size  int64
	Files []DedupFile
}

// DedupSnapshot is an immutable dedup result generation.
type DedupSnapshot struct {
	Root           pathloc.Path // scan path (unchanged after trim)
	DisplayRoot    pathloc.Path // results view root; zero means same as Root
	Phase          DedupPhase
	Groups         []DedupGroup
	Walked         int
	Hashed         int
	HashTotal      int
	HashBytesTotal int64  // total bytes among hash candidates (confirm gate + progress context)
	HashedBytes    int64  // bytes hashed so far: completed candidates + in-progress partial reads
	Current        string // rel directory of the tracked in-progress file (progress label)
	// CurrentFile is the filename (no path) of the tracked in-progress file,
	// set only when its size is at least DedupOptions.FileProgressBytes.
	CurrentFile     string
	CurrentFileSize int64
	CurrentFileDone int64
	Err             string
}

// WithoutPaths returns a copy of the snapshot with the given absolute paths removed;
// groups that fall below two members are dropped entirely.
func (s DedupSnapshot) WithoutPaths(removed map[string]bool) DedupSnapshot {
	out := s
	out.Groups = nil
	for _, g := range s.Groups {
		var kept []DedupFile
		for _, f := range g.Files {
			if !removed[f.Abs.String()] {
				kept = append(kept, f)
			}
		}
		if len(kept) >= 2 {
			ng := g
			ng.Files = kept
			out.Groups = append(out.Groups, ng)
		}
	}
	return out
}

func (s *DedupSession) walkRoot(ctx context.Context) ([]FileRecord, error) {
	walkOpts := s.opts.Walk
	walkOpts.SkipSymlinks = true
	var pubMu sync.Mutex
	var lastPub time.Time
	walkOpts.OnFile = func(walked int) {
		pubMu.Lock()
		if !lastPub.IsZero() && time.Since(lastPub) < dedupProgressInterval {
			pubMu.Unlock()
			return
		}
		lastPub = time.Now()
		pubMu.Unlock()
		s.publish(DedupSnapshot{
			Root:   s.root,
			Phase:  DedupWalking,
			Walked: walked,
		})
	}
	return WalkRoot(ctx, s.root, walkOpts)
}

// DedupOptions configures a dedup session.
type DedupOptions struct {
	Walk         WalkOptions
	HashWorkers  int
	ReadBuffer   []byte
	MaxHashBytes int64
	// ConfirmHashBytes, when >0, pauses before hashing (phase DedupAwaitConfirm)
	// once the total byte size of hash candidates exceeds it, until Confirm() is called.
	ConfirmHashBytes int64
	// FileProgressBytes, when >0, exposes per-file hashing progress
	// (CurrentFile/CurrentFileSize/CurrentFileDone) for tracked files at or
	// above this size. Zero disables per-file progress.
	FileProgressBytes int64
	OnUpdate          func(DedupSnapshot)
}

// DedupSession walks one root, size-prefilters, hashes candidates, and groups duplicates.
type DedupSession struct {
	root pathloc.Path
	opts DedupOptions

	cancel context.CancelFunc
	wg     sync.WaitGroup

	confirm     chan struct{}
	confirmOnce sync.Once

	snap atomic.Pointer[DedupSnapshot]
}

// StartDedup begins scanning root for duplicate files in the background.
func StartDedup(ctx context.Context, root pathloc.Path, opts DedupOptions) *DedupSession {
	ctx, cancel := context.WithCancel(ctx)
	s := &DedupSession{
		root:    root,
		opts:    opts,
		cancel:  cancel,
		confirm: make(chan struct{}),
	}
	s.snap.Store(&DedupSnapshot{Root: root, Phase: DedupWalking})
	s.wg.Add(1)
	go s.run(ctx)
	return s
}

// Snapshot returns the latest dedup state.
func (s *DedupSession) Snapshot() DedupSnapshot {
	if p := s.snap.Load(); p != nil {
		return *p
	}
	return DedupSnapshot{}
}

// Close cancels and waits for the worker.
func (s *DedupSession) Close() {
	s.cancel()
	s.wg.Wait()
}

// Confirm resumes hashing after a DedupAwaitConfirm pause. Extra calls are no-ops.
func (s *DedupSession) Confirm() {
	s.confirmOnce.Do(func() { close(s.confirm) })
}

func (s *DedupSession) publish(snap DedupSnapshot) {
	cp := snap
	s.snap.Store(&cp)
	if s.opts.OnUpdate != nil {
		s.opts.OnUpdate(cp)
	}
}

func (s *DedupSession) run(ctx context.Context) {
	defer s.wg.Done()

	if s.root.IsRemote() {
		s.publish(DedupSnapshot{Root: s.root, Phase: DedupError, Err: errRemoteNotSupported.Error()})
		return
	}

	files, err := s.walkRoot(ctx)
	if err != nil {
		if ctx.Err() != nil {
			s.publish(DedupSnapshot{Root: s.root, Phase: DedupCanceled})
			return
		}
		s.publish(DedupSnapshot{Root: s.root, Phase: DedupError, Err: err.Error()})
		return
	}

	// Size prefilter: a file whose byte size is unique in the tree provably has no
	// duplicate, so it is never opened. Only files sharing a size are hash candidates.
	// All zero-byte files collide on size 0 and group together; the dedup view
	// hides them by default via its ignore-empty toggle (DedupEntriesFromSnapshot).
	bySize := map[int64][]FileRecord{}
	for _, f := range files {
		bySize[f.Size] = append(bySize[f.Size], f)
	}
	var candidates []FileRecord
	for _, group := range bySize {
		if len(group) > 1 {
			candidates = append(candidates, group...)
		}
	}

	if len(candidates) == 0 {
		s.publish(DedupSnapshot{Root: s.root, Phase: DedupDone, Walked: len(files)})
		return
	}

	var candidateBytes int64
	for _, f := range candidates {
		candidateBytes += f.Size
	}

	// Gate the expensive hashing phase behind confirmation for large candidate sets.
	if s.opts.ConfirmHashBytes > 0 && candidateBytes > s.opts.ConfirmHashBytes {
		s.publish(DedupSnapshot{
			Root:           s.root,
			Phase:          DedupAwaitConfirm,
			Walked:         len(files),
			HashTotal:      len(candidates),
			HashBytesTotal: candidateBytes,
		})
		select {
		case <-ctx.Done():
			s.publish(DedupSnapshot{Root: s.root, Phase: DedupCanceled})
			return
		case <-s.confirm:
		}
	}

	workers := max(s.opts.HashWorkers, 1)
	bufSize := len(s.opts.ReadBuffer)
	if bufSize == 0 {
		bufSize = 256 * 1024
	}

	s.publish(DedupSnapshot{
		Root:           s.root,
		Phase:          DedupHashing,
		Walked:         len(files),
		HashTotal:      len(candidates),
		HashBytesTotal: candidateBytes,
	})

	// Each worker writes only to its own unique index, so no mutex is needed.
	hashes := make([][32]byte, len(candidates))
	hashErr := make([]bool, len(candidates))

	// Progress is byte-based so large files advance the bar smoothly instead of
	// jumping one file at a time. One mutex guards the tracker; contention is one
	// lock per read-buffer chunk per worker, which is negligible.
	var pubMu sync.Mutex
	var lastPub time.Time
	doneFiles := 0
	var doneBytes int64
	inProgress := map[int]int64{} // candidate idx -> bytes hashed so far
	publishProgress := func() {
		pubMu.Lock()
		if !lastPub.IsZero() && time.Since(lastPub) < dedupProgressInterval {
			pubMu.Unlock()
			return
		}
		lastPub = time.Now()
		// Track the largest in-progress candidate (lowest index tiebreak): it is
		// stable for as long as it hashes, so the label does not jitter between
		// whichever worker last reported.
		tracked := -1
		hashedBytes := doneBytes
		for idx, read := range inProgress {
			hashedBytes += read
			if tracked < 0 || candidates[idx].Size > candidates[tracked].Size ||
				(candidates[idx].Size == candidates[tracked].Size && idx < tracked) {
				tracked = idx
			}
		}
		snap := DedupSnapshot{
			Root:           s.root,
			Phase:          DedupHashing,
			Walked:         len(files),
			Hashed:         doneFiles,
			HashTotal:      len(candidates),
			HashBytesTotal: candidateBytes,
			HashedBytes:    hashedBytes,
		}
		if tracked >= 0 {
			snap.Current = RelDir(candidates[tracked].Rel)
			if s.opts.FileProgressBytes > 0 && candidates[tracked].Size >= s.opts.FileProgressBytes {
				snap.CurrentFile = RelBase(candidates[tracked].Rel)
				snap.CurrentFileSize = candidates[tracked].Size
				snap.CurrentFileDone = inProgress[tracked]
			}
		}
		// Publish while still holding the lock so snapshots cannot be stored out
		// of order (HashedBytes must never regress); publish is an atomic store
		// plus a coalesced wake, so this is cheap.
		s.publish(snap)
		pubMu.Unlock()
	}

	jobCh := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		buf := make([]byte, bufSize)
		go func() {
			defer wg.Done()
			for idx := range jobCh {
				if ctx.Err() != nil {
					return
				}
				pubMu.Lock()
				inProgress[idx] = 0
				pubMu.Unlock()
				sum, err := HashFile(ctx, candidates[idx].Abs, buf, s.opts.MaxHashBytes, func(read int64) {
					pubMu.Lock()
					inProgress[idx] = read
					pubMu.Unlock()
					publishProgress()
				})
				if err != nil {
					hashErr[idx] = true
				} else {
					hashes[idx] = sum
				}
				// Charge the full size even on error so HashedBytes stays
				// monotonic and reaches HashBytesTotal.
				pubMu.Lock()
				delete(inProgress, idx)
				doneFiles++
				doneBytes += candidates[idx].Size
				pubMu.Unlock()
				publishProgress()
			}
		}()
	}
	for idx := range candidates {
		if ctx.Err() != nil {
			break
		}
		jobCh <- idx
	}
	close(jobCh)
	wg.Wait()

	if ctx.Err() != nil {
		s.publish(DedupSnapshot{Root: s.root, Phase: DedupCanceled})
		return
	}

	s.publish(DedupSnapshot{
		Root:           s.root,
		Phase:          DedupDone,
		Groups:         groupByHash(candidates, hashes, hashErr),
		Walked:         len(files),
		Hashed:         len(candidates),
		HashTotal:      len(candidates),
		HashBytesTotal: candidateBytes,
		HashedBytes:    candidateBytes,
	})
}

func groupByHash(candidates []FileRecord, hashes [][32]byte, hashErr []bool) []DedupGroup {
	byHash := map[[32]byte][]int{}
	for idx := range candidates {
		if hashErr[idx] {
			continue
		}
		byHash[hashes[idx]] = append(byHash[hashes[idx]], idx)
	}
	var groups []DedupGroup
	for h, idxs := range byHash {
		if len(idxs) < 2 {
			continue
		}
		files := make([]DedupFile, 0, len(idxs))
		for _, idx := range idxs {
			files = append(files, DedupFile{Rel: candidates[idx].Rel, Abs: candidates[idx].Abs})
		}
		slices.SortFunc(files, func(a, b DedupFile) int { return cmp.Compare(a.Rel, b.Rel) })
		groups = append(groups, DedupGroup{Hash: h, Size: candidates[idxs[0]].Size, Files: files})
	}
	slices.SortFunc(groups, DedupGroupBySize)
	return groups
}

// DedupGroupBySize orders duplicate groups "most space wasted" first: largest
// file size first, hash as a stable tiebreak. Shared by the backend group build
// and the view's sort toggle so ordering has one definition.
func DedupGroupBySize(a, b DedupGroup) int {
	if a.Size != b.Size {
		return cmp.Compare(b.Size, a.Size) // largest first
	}
	return bytes.Compare(a.Hash[:], b.Hash[:])
}
