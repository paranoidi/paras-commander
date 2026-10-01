package compare

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"hash"
	"io"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// dedupProgressInterval rate-limits dedup walk and hashing progress publishes.
// ponytail: fixed 500ms; promote to config if someone wants to tune it.
// A var only so tests can publish every update.
var dedupProgressInterval = 500 * time.Millisecond

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
	Scope          []string     // DedupOptions.Walk.Only of the scan; empty = whole Root
	DisplayRoot    pathloc.Path // results view root; zero means same as Root
	Phase          DedupPhase
	Groups         []DedupGroup
	Walked         int
	Hashed         int
	HashTotal      int
	HashBytesTotal int64     // total bytes among hash candidates (confirm gate + progress context)
	HashStarted    time.Time // wall time hashing began, after the confirm gate; basis for the progress ETA
	HashedBytes    int64     // bytes hashed so far: completed candidates + in-progress partial reads
	Current        string    // rel directory of the tracked in-progress file (progress label)
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
	// ChunkBytes compares same-size files this many bytes at a time so a file
	// can be dropped as soon as its prefix diverges from every other file in
	// its partition. Zero or negative disables chunking (whole file in one round).
	ChunkBytes int64
	// ConfirmHashBytes, when >0, pauses before hashing (phase DedupAwaitConfirm)
	// once the total byte size of hash candidates exceeds it, until Confirm() is called.
	ConfirmHashBytes int64
	// FileProgressBytes, when >0, exposes per-file hashing progress
	// (CurrentFile/CurrentFileSize/CurrentFileDone) for tracked files at or
	// above this size. Zero disables per-file progress.
	FileProgressBytes int64
	// CacheFile, when non-empty, persists full-file hashes keyed by path, size
	// and mtime so later scans skip unchanged files. Empty disables the cache.
	CacheFile string
	OnUpdate  func(DedupSnapshot)
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
	s.snap.Store(&DedupSnapshot{Root: root, Scope: opts.Walk.Only, Phase: DedupWalking})
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
	cp.Scope = s.opts.Walk.Only
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
	var sizeGroups [][]int // candidate indices per size class; each hashes as one unit
	for size, group := range bySize {
		if len(group) < 2 {
			continue
		}
		if s.opts.MaxHashBytes > 0 && size > s.opts.MaxHashBytes {
			continue // oversize files are never opened (same outcome as the old per-file error)
		}
		idxs := make([]int, 0, len(group))
		for _, f := range group {
			idxs = append(idxs, len(candidates))
			candidates = append(candidates, f)
		}
		sizeGroups = append(sizeGroups, idxs)
	}

	if len(candidates) == 0 {
		s.publish(DedupSnapshot{Root: s.root, Phase: DedupDone, Walked: len(files)})
		return
	}

	// Each size group is resolved by exactly one worker and groups have disjoint
	// candidate indices, so these slices need no mutex. hashOK is set only for
	// files read to EOF: partial prefix digests of early-bailed files must never
	// reach groupByHash (files from different size groups can share a prefix).
	hashes := make([][32]byte, len(candidates))
	hashOK := make([]bool, len(candidates))
	hashers := make([]hash.Hash, len(candidates))

	// Cache hits (same path, size, mtime) are prefilled and never read; only
	// misses count toward the bytes to hash.
	var cache map[string]hashCacheEntry
	var walked map[string]FileRecord
	hit := make([]bool, len(candidates))
	hits := 0
	if s.opts.CacheFile != "" {
		cache = loadHashCache(s.opts.CacheFile)
		walked = make(map[string]FileRecord, len(files))
		for _, f := range files {
			walked[f.Abs.String()] = f
		}
		for idx, f := range candidates {
			if e, ok := cache[f.Abs.String()]; ok && e.Size == f.Size && e.ModTime == f.ModTime {
				hit[idx], hashOK[idx], hashes[idx] = true, true, e.Hash
				hits++
			}
		}
	}
	var candidateBytes int64
	for idx, f := range candidates {
		if !hit[idx] {
			candidateBytes += f.Size
		}
	}
	saveCache := func() {
		if s.opts.CacheFile == "" {
			return
		}
		fresh := map[string]hashCacheEntry{}
		for idx, f := range candidates {
			if hashOK[idx] {
				fresh[f.Abs.String()] = hashCacheEntry{Size: f.Size, ModTime: f.ModTime, Hash: hashes[idx]}
			}
		}
		saveHashCache(s.opts.CacheFile, mergeHashCache(cache, s.root.String(), walked, fresh))
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

	hashStarted := time.Now()
	s.publish(DedupSnapshot{
		Root:           s.root,
		Phase:          DedupHashing,
		Walked:         len(files),
		Hashed:         hits,
		HashTotal:      len(candidates),
		HashBytesTotal: candidateBytes,
		HashStarted:    hashStarted,
	})

	chunk := s.opts.ChunkBytes
	if chunk <= 0 {
		chunk = math.MaxInt64
	}

	// Progress is byte-based so large files advance the bar smoothly instead of
	// jumping one file at a time. One mutex guards the tracker; contention is one
	// lock per read-buffer chunk per worker, which is negligible.
	var pubMu sync.Mutex
	var lastPub time.Time
	doneFiles := hits
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
			HashStarted:    hashStarted,
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

	// resolve retires a candidate (EOF, early bail, or read error). Charging the
	// full size even for bailed/errored files keeps HashedBytes monotonic and
	// reaching HashBytesTotal; skipped bytes just jump the bar forward.
	resolve := func(idx int) {
		pubMu.Lock()
		delete(inProgress, idx)
		doneFiles++
		doneBytes += candidates[idx].Size
		pubMu.Unlock()
		publishProgress()
	}

	// processGroup compares one size class chunk by chunk: each round hashes the
	// next chunk of every member, then splits the partition by prefix digest.
	// Singleton partitions are provably unique and stop reading immediately;
	// partitions that reach EOF are duplicate sets and their hashers now hold the
	// true full-file SHA-256.
	// ponytail: one worker walks a whole size group serially; parallelize files
	// inside a giant group if that ever shows up in practice.
	type partition struct {
		idxs   []int
		offset int64
	}
	// full forces whole-file digests (chunk = MaxInt64, singletons included) so
	// uncached members of a partly cached group compare against cached digests.
	processGroup := func(group []int, buf []byte, full bool) {
		chunk := chunk
		if full {
			chunk = math.MaxInt64
		}
		size := candidates[group[0]].Size
		pubMu.Lock()
		for _, idx := range group {
			hashers[idx] = sha256.New()
			inProgress[idx] = 0
		}
		pubMu.Unlock()
		stack := []partition{{idxs: group}}
		for len(stack) > 0 {
			if ctx.Err() != nil {
				return
			}
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if p.offset >= size {
				for _, idx := range p.idxs {
					copy(hashes[idx][:], hashers[idx].Sum(nil))
					hashOK[idx] = true
					resolve(idx)
				}
				continue
			}
			end := p.offset + chunk
			if end > size || end < 0 { // end < 0: offset+MaxInt64 overflow
				end = size
			}
			byKey := map[string][]int{}
			for _, idx := range p.idxs {
				err := hashChunk(ctx, candidates[idx].Abs, hashers[idx], buf, p.offset, end-p.offset, func(read int64) {
					pubMu.Lock()
					inProgress[idx] = p.offset + read
					pubMu.Unlock()
					publishProgress()
				})
				if err != nil {
					resolve(idx) // unreadable: drop it, keep comparing the rest
					continue
				}
				key := string(hashers[idx].Sum(nil))
				byKey[key] = append(byKey[key], idx)
			}
			for _, sub := range byKey {
				if len(sub) == 1 && !full {
					resolve(sub[0]) // unique prefix: no duplicate possible, skip the rest
					continue
				}
				stack = append(stack, partition{idxs: sub, offset: end})
			}
		}
	}

	jobCh := make(chan []int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		buf := make([]byte, bufSize)
		go func() {
			defer wg.Done()
			for group := range jobCh {
				if ctx.Err() != nil {
					return
				}
				var misses []int
				for _, idx := range group {
					if !hit[idx] {
						misses = append(misses, idx)
					}
				}
				switch {
				case len(misses) == len(group):
					processGroup(group, buf, false)
				case len(misses) > 0:
					// ponytail: mixed groups lose early-bail for uncached members; cache prefix digests if that bites.
					processGroup(misses, buf, true)
				}
			}
		}()
	}
	for _, group := range sizeGroups {
		if ctx.Err() != nil {
			break
		}
		jobCh <- group
	}
	close(jobCh)
	wg.Wait()

	// ponytail: last-writer-wins across concurrent pc processes; entries only for files hashed to EOF (duplicates), so the file stays small.
	saveCache()
	if ctx.Err() != nil {
		s.publish(DedupSnapshot{Root: s.root, Phase: DedupCanceled})
		return
	}

	s.publish(DedupSnapshot{
		Root:           s.root,
		Phase:          DedupDone,
		Groups:         groupByHash(candidates, hashes, hashOK),
		Walked:         len(files),
		Hashed:         len(candidates),
		HashTotal:      len(candidates),
		HashBytesTotal: candidateBytes,
		HashedBytes:    candidateBytes,
	})
}

// hashChunk feeds bytes [offset, offset+n) of loc into h, reporting cumulative
// bytes read within the chunk via onRead after every buffer read.
func hashChunk(ctx context.Context, loc pathloc.Path, h hash.Hash, buf []byte, offset, n int64, onRead func(read int64)) error {
	be, err := fsbackend.Default().Backend(loc)
	if err != nil {
		return err
	}
	rc, err := be.OpenRead(ctx, loc)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	if offset > 0 {
		if sk, ok := rc.(io.Seeker); ok {
			if _, err := sk.Seek(offset, io.SeekStart); err != nil {
				return err
			}
		} else if _, err := io.CopyN(io.Discard, rc, offset); err != nil {
			return err
		}
	}
	var read int64
	for read < n {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		want := min(n-read, int64(len(buf)))
		nr, readErr := rc.Read(buf[:want])
		if nr > 0 {
			if _, werr := h.Write(buf[:nr]); werr != nil {
				return werr
			}
			read += int64(nr)
			onRead(read)
		}
		if readErr == io.EOF {
			if read < n {
				return io.ErrUnexpectedEOF // file shrank since the walk
			}
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func groupByHash(candidates []FileRecord, hashes [][32]byte, hashOK []bool) []DedupGroup {
	byHash := map[[32]byte][]int{}
	for idx := range candidates {
		if !hashOK[idx] {
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
