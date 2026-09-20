package jobs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestConcurrentScanFlipPauseDequeueAllJobs characterizes R04-002: Job.Status and
// FinishedAt must have a single happens-before relationship. Scan flip writes them
// under State.mu; pause/resume/dequeue used to touch the same fields under Queue.mu
// only; AllJobs copies them after releasing Queue.mu. Run under -race.
func TestConcurrentScanFlipPauseDequeueAllJobs(t *testing.T) {
	s := NewState()
	s.SetScanConfig(ScanConfig{ProgressMinInterval: time.Hour})

	const n = 24
	type scanCtl struct {
		first chan struct{}
		done  chan struct{}
	}
	ctls := make([]*scanCtl, n)
	for i := range ctls {
		ctls[i] = &scanCtl{first: make(chan struct{}), done: make(chan struct{})}
	}
	var next int
	var nextMu sync.Mutex
	s.SetScanFunc(func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks ScanWalkHooks) PlanProducer {
		nextMu.Lock()
		i := next
		next++
		nextMu.Unlock()
		ctl := ctls[i]
		return PlanProducer{
			Items:     make(chan ops.PlanItem),
			FirstItem: ctl.first,
			Totals:    func() (int, int, int64) { return 0, 0, 0 },
			Done:      ctl.done,
			Err:       func() error { return ctx.Err() },
		}
	})

	jobs := make([]*Job, n)
	for i := 0; i < n; i++ {
		jobs[i] = &Job{
			ID:          fmt.Sprintf("lock-%d", i),
			Type:        TypeCopy,
			Status:      StatusScanning,
			Sources:     pathloc.PathsForTest("/willow"),
			Destination: pathloc.MustParse("/harbor"),
		}
		s.AddJob(jobs[i])
	}

	// Drain the event channel so scan-flip emits cannot block on a full buffer.
	stopDrain := make(chan struct{})
	var drainWG sync.WaitGroup
	drainWG.Add(1)
	go func() {
		defer drainWG.Done()
		for {
			select {
			case <-stopDrain:
				for {
					select {
					case <-s.Events():
					default:
						return
					}
				}
			case <-s.Events():
			}
		}
	}()

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 400; i++ {
			for _, j := range s.AllJobs() {
				if j == nil {
					continue
				}
				_ = j.Status
				_ = j.FinishedAt
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 400; i++ {
			id := jobs[i%n].ID
			s.PauseQueuedJob(id)
			s.ResumeJob(id)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			_ = s.dequeueJob()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < n; i++ {
			close(ctls[i].first)
		}
		for i := 0; i < n; i++ {
			close(ctls[i].done)
		}
	}()

	close(start)
	wg.Wait()
	close(stopDrain)
	drainWG.Wait()
}
