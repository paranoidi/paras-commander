package compare

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Options configures a compare session.
type Options struct {
	Walk        WalkOptions
	HashWorkers int
	// ReadBufferBytes sizes each worker's read buffer; zero uses the built-in default.
	ReadBufferBytes int
	OnUpdate        func(Snapshot)
}

// hashJob is one unit of content work: a same-path pair compared with SameContent (pair set,
// loc primary, other secondary) or one unpaired file hashed for relocation matching (side).
type hashJob struct {
	side  int // 0 primary, 1 secondary; unpaired files only
	pair  bool
	rel   string
	loc   pathloc.Path
	other pathloc.Path
	size  int64
}

// files is how many files the job reads: progress counts a pair as two.
func (j hashJob) files() int {
	return len(j.sides())
}

// sides are the walk sides (0 primary, 1 secondary) the job reads.
func (j hashJob) sides() []int {
	if j.pair {
		return []int{0, 1}
	}
	return []int{j.side}
}

// Session runs walk + hash + classify in the background.
type Session struct {
	primaryRoot   pathloc.Path
	secondaryRoot pathloc.Path
	opts          Options

	cancel context.CancelFunc
	wg     sync.WaitGroup

	snap atomic.Pointer[Snapshot]

	forgetMu         sync.Mutex
	primaryRemoved   []string
	secondaryRemoved []string
}

// Start begins comparing primary and secondary roots.
func Start(ctx context.Context, primary, secondary pathloc.Path, opts Options) *Session {
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{
		primaryRoot:   primary,
		secondaryRoot: secondary,
		opts:          opts,
		cancel:        cancel,
	}
	initial := &Snapshot{
		PrimaryRoot:   primary,
		SecondaryRoot: secondary,
		Phase:         PhaseWalking,
	}
	s.snap.Store(initial)
	s.wg.Add(1)
	go s.run(ctx)
	return s
}

// Snapshot returns the latest compare state.
func (s *Session) Snapshot() Snapshot {
	if p := s.snap.Load(); p != nil {
		return *p
	}
	return Snapshot{}
}

// Close cancels and waits for the worker.
func (s *Session) Close() {
	s.cancel()
	s.wg.Wait()
}

// Forget removes deleted files (rels relative to each root) from every snapshot published
// from now on, including the current one, so no re-walk or re-hash is needed.
func (s *Session) Forget(primaryRels, secondaryRels []string) {
	s.forgetMu.Lock()
	s.primaryRemoved = append(s.primaryRemoved, primaryRels...)
	s.secondaryRemoved = append(s.secondaryRemoved, secondaryRels...)
	s.forgetMu.Unlock()
	if p := s.snap.Load(); p != nil {
		s.publish(*p)
	}
}

func (s *Session) publish(snap Snapshot) {
	cp := snap
	s.forgetMu.Lock()
	// Store under the lock so a worker pruned with an older removed list cannot land after Forget.
	cp.Rows = PruneRows(cp.Rows, s.primaryRemoved, s.secondaryRemoved)
	s.snap.Store(&cp)
	s.forgetMu.Unlock()
	if s.opts.OnUpdate != nil {
		s.opts.OnUpdate(cp)
	}
}

func (s *Session) run(ctx context.Context) {
	defer s.wg.Done()

	if !s.primaryRoot.IsLocal() || !s.secondaryRoot.IsLocal() {
		s.publish(Snapshot{
			PrimaryRoot:   s.primaryRoot,
			SecondaryRoot: s.secondaryRoot,
			Phase:         PhaseError,
			Err:           errRemoteNotSupported.Error(),
		})
		return
	}

	type walkResult struct {
		files []FileRecord
		err   error
	}
	pCh := make(chan walkResult, 1)
	sCh := make(chan walkResult, 1)
	go func() {
		files, err := WalkRoot(ctx, s.primaryRoot, s.opts.Walk)
		pCh <- walkResult{files: files, err: err}
	}()
	go func() {
		files, err := WalkRoot(ctx, s.secondaryRoot, s.opts.Walk)
		sCh <- walkResult{files: files, err: err}
	}()

	var pRes, sRes walkResult
	select {
	case <-ctx.Done():
		s.publish(Snapshot{PrimaryRoot: s.primaryRoot, SecondaryRoot: s.secondaryRoot, Phase: PhaseCanceled})
		return
	case pRes = <-pCh:
	}
	select {
	case <-ctx.Done():
		s.publish(Snapshot{PrimaryRoot: s.primaryRoot, SecondaryRoot: s.secondaryRoot, Phase: PhaseCanceled})
		return
	case sRes = <-sCh:
	}

	if ctx.Err() != nil {
		s.publish(Snapshot{PrimaryRoot: s.primaryRoot, SecondaryRoot: s.secondaryRoot, Phase: PhaseCanceled})
		return
	}
	if pRes.err != nil {
		s.publish(Snapshot{
			PrimaryRoot:   s.primaryRoot,
			SecondaryRoot: s.secondaryRoot,
			Phase:         PhaseError,
			Err:           pRes.err.Error(),
		})
		return
	}
	if sRes.err != nil {
		s.publish(Snapshot{
			PrimaryRoot:     s.primaryRoot,
			SecondaryRoot:   s.secondaryRoot,
			Phase:           PhaseError,
			WalkedPrimary:   len(pRes.files),
			WalkedSecondary: len(sRes.files),
			Err:             sRes.err.Error(),
		})
		return
	}

	primary := pRes.files
	secondary := sRes.files
	// Per-side state indexed by walk side (0 primary, 1 secondary). A pair job's error is
	// recorded on the primary side.
	hashes := [2]map[string][32]byte{{}, {}}
	errs := [2]map[string]string{{}, {}}
	pairSame := make(map[string]bool)
	classify := func() []Row { return Classify(primary, secondary, hashes[0], hashes[1], pairSame, errs[0], errs[1]) }

	jobs := hashJobsNeeded(primary, secondary)
	hashTotal := 0
	for _, j := range jobs {
		hashTotal += j.files()
	}
	if len(jobs) == 0 {
		s.publish(Snapshot{
			PrimaryRoot:     s.primaryRoot,
			SecondaryRoot:   s.secondaryRoot,
			Phase:           PhaseDone,
			Rows:            classify(),
			WalkedPrimary:   len(primary),
			WalkedSecondary: len(secondary),
		})
		return
	}

	workers := s.opts.HashWorkers
	if workers < 1 {
		workers = 1
	}
	bufSize := s.opts.ReadBufferBytes
	if bufSize <= 0 {
		bufSize = config.DefaultCompareReadBufferKiB * 1024
	}

	s.publish(Snapshot{
		PrimaryRoot:     s.primaryRoot,
		SecondaryRoot:   s.secondaryRoot,
		Phase:           PhaseHashing,
		Rows:            classify(),
		WalkedPrimary:   len(primary),
		WalkedSecondary: len(secondary),
		HashTotal:       hashTotal,
	})

	jobCh := make(chan hashJob)
	var wg sync.WaitGroup
	var hashed atomic.Int32
	var mu sync.Mutex
	hashing := [2]map[string]struct{}{{}, {}}
	publishRows := func(rows []Row, h int) {
		s.publish(Snapshot{
			PrimaryRoot:     s.primaryRoot,
			SecondaryRoot:   s.secondaryRoot,
			Phase:           PhaseHashing,
			Rows:            rows,
			WalkedPrimary:   len(primary),
			WalkedSecondary: len(secondary),
			Hashed:          h,
			HashTotal:       hashTotal,
		})
	}

	// One publisher classifies and publishes on a ticker, so cost is O(N x ticks), not O(N^2).
	stopPub := make(chan struct{})
	pubDone := make(chan struct{})
	go func() {
		defer close(pubDone)
		t := time.NewTicker(progressInterval)
		defer t.Stop()
		for {
			select {
			case <-stopPub:
				return
			case <-t.C:
				mu.Lock()
				rows := MarkHashing(classify(), hashing[0], hashing[1])
				h := int(hashed.Load())
				mu.Unlock()
				publishRows(rows, h)
			}
		}
	}()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		buf := make([]byte, bufSize)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if ctx.Err() != nil {
					return
				}
				sides := job.sides()
				mu.Lock()
				for _, side := range sides {
					hashing[side][job.rel] = struct{}{}
				}
				mu.Unlock()

				var same bool
				var sum [32]byte
				var err error
				if job.pair {
					same, err = SameContent(ctx, job.loc, job.other, buf, nil)
				} else {
					sum, err = HashFile(ctx, job.loc, buf)
				}
				mu.Lock()
				for _, side := range sides {
					delete(hashing[side], job.rel)
				}
				switch {
				case err != nil:
					errs[sides[0]][job.rel] = err.Error()
				case job.pair:
					pairSame[job.rel] = same
				default:
					hashes[job.side][job.rel] = sum
				}
				hashed.Add(int32(len(sides)))
				mu.Unlock()
			}
		}()
	}

	for _, job := range jobs {
		if ctx.Err() != nil {
			break
		}
		jobCh <- job
	}
	close(jobCh)
	wg.Wait()
	close(stopPub)
	<-pubDone // no stale PhaseHashing snapshot may land after the final publish

	if ctx.Err() != nil {
		s.publish(Snapshot{PrimaryRoot: s.primaryRoot, SecondaryRoot: s.secondaryRoot, Phase: PhaseCanceled})
		return
	}

	mu.Lock()
	rows := classify()
	mu.Unlock()
	s.publish(Snapshot{
		PrimaryRoot:     s.primaryRoot,
		SecondaryRoot:   s.secondaryRoot,
		Phase:           PhaseDone,
		Rows:            rows,
		WalkedPrimary:   len(primary),
		WalkedSecondary: len(secondary),
		Hashed:          hashTotal,
		HashTotal:       hashTotal,
	})
}
