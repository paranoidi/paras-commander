package compare

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Options configures a compare session.
type Options struct {
	Walk         WalkOptions
	HashWorkers  int
	ReadBuffer   []byte
	MaxHashBytes int64
	OnUpdate     func(Snapshot)
}

type hashJob struct {
	side int // 0 primary, 1 secondary
	rel  string
	loc  pathloc.Path
}

// Session runs walk + hash + classify in the background.
type Session struct {
	primaryRoot   pathloc.Path
	secondaryRoot pathloc.Path
	opts          Options

	cancel context.CancelFunc
	wg     sync.WaitGroup

	snap atomic.Pointer[Snapshot]
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

func (s *Session) publish(snap Snapshot) {
	cp := snap
	s.snap.Store(&cp)
	if s.opts.OnUpdate != nil {
		s.opts.OnUpdate(cp)
	}
}

func (s *Session) run(ctx context.Context) {
	defer s.wg.Done()

	if s.primaryRoot.IsRemote() || s.secondaryRoot.IsRemote() {
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
	pHash := make(map[string][32]byte)
	sHash := make(map[string][32]byte)
	pErr := make(map[string]string)
	sErr := make(map[string]string)

	jobs := make([]hashJob, 0, len(primary)+len(secondary))
	for _, f := range primary {
		jobs = append(jobs, hashJob{side: 0, rel: f.Rel, loc: f.Abs})
	}
	for _, f := range secondary {
		jobs = append(jobs, hashJob{side: 1, rel: f.Rel, loc: f.Abs})
	}

	workers := s.opts.HashWorkers
	if workers < 1 {
		workers = 1
	}
	bufSize := len(s.opts.ReadBuffer)
	if bufSize == 0 {
		bufSize = 256 * 1024
	}

	s.publish(Snapshot{
		PrimaryRoot:     s.primaryRoot,
		SecondaryRoot:   s.secondaryRoot,
		Phase:           PhaseHashing,
		Rows:            Classify(primary, secondary, pHash, sHash, pErr, sErr),
		WalkedPrimary:   len(primary),
		WalkedSecondary: len(secondary),
		HashTotal:       len(jobs),
	})

	jobCh := make(chan hashJob)
	var wg sync.WaitGroup
	var hashed atomic.Int32
	var mu sync.Mutex

	const flushInterval = 50

	flush := func() {
		mu.Lock()
		rows := Classify(primary, secondary, pHash, sHash, pErr, sErr)
		h := int(hashed.Load())
		mu.Unlock()
		s.publish(Snapshot{
			PrimaryRoot:     s.primaryRoot,
			SecondaryRoot:   s.secondaryRoot,
			Phase:           PhaseHashing,
			Rows:            rows,
			WalkedPrimary:   len(primary),
			WalkedSecondary: len(secondary),
			Hashed:          h,
			HashTotal:       len(jobs),
		})
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		buf := make([]byte, bufSize)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if ctx.Err() != nil {
					return
				}
				sum, err := HashFile(ctx, job.loc, buf, s.opts.MaxHashBytes, nil)
				mu.Lock()
				if err != nil {
					if job.side == 0 {
						pErr[job.rel] = err.Error()
					} else {
						sErr[job.rel] = err.Error()
					}
				} else if job.side == 0 {
					pHash[job.rel] = sum
				} else {
					sHash[job.rel] = sum
				}
				mu.Unlock()
				if n := hashed.Add(1); n%flushInterval == 0 {
					flush()
				}
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

	if ctx.Err() != nil {
		s.publish(Snapshot{PrimaryRoot: s.primaryRoot, SecondaryRoot: s.secondaryRoot, Phase: PhaseCanceled})
		return
	}

	mu.Lock()
	rows := Classify(primary, secondary, pHash, sHash, pErr, sErr)
	mu.Unlock()
	s.publish(Snapshot{
		PrimaryRoot:     s.primaryRoot,
		SecondaryRoot:   s.secondaryRoot,
		Phase:           PhaseDone,
		Rows:            rows,
		WalkedPrimary:   len(primary),
		WalkedSecondary: len(secondary),
		Hashed:          len(jobs),
		HashTotal:       len(jobs),
	})
}
