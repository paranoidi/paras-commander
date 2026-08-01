package scan

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fswalk"
)

func TestHiddenExpansionStepsWithoutBlockingCoordinator(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("g"), 0o644); err != nil {
		t.Fatal(err)
	}

	var indexDone atomic.Bool
	var batchEvents atomic.Int32
	coord := NewCoordinator(fswalk.Params{InitialWorkers: 1, MaxWorkers: 2, AdaptIntervalMS: 500}, func(ev Event) {
		if ev.IndexFinished {
			indexDone.Store(true)
		}
		if len(ev.BatchAdded) > 0 {
			batchEvents.Add(1)
		}
	})

	coord.Start(StartOpts{
		Gen:         1,
		DisplayRoot: root,
		Roots:       []string{root},
		Walk:        fswalk.Params{InitialWorkers: 1, MaxWorkers: 2, AdaptIntervalMS: 500},
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if indexDone.Load() {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !indexDone.Load() {
		t.Fatal("initial index did not finish")
	}

	before := batchEvents.Load()
	coord.SetIncludeHidden(true)

	workDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(workDeadline) {
		if batchEvents.Load() > before {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if batchEvents.Load() <= before {
		t.Fatal("coordinator did not emit hidden batches after include-hidden enable")
	}

	coord.Cancel()
}

func TestSessionProcessHiddenOneStepSplicesOneBatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := newHiddenWorkTestSession(root)
	for i := range hiddenFilesSpliceBatch + 100 {
		s.hidden.mergeSkipped(nil, []string{filepath.Join(root, ".f_"+hiddenTestItoa(i))})
	}
	before := s.idx.Len()
	s.processHiddenOneStep()
	if got := s.idx.Len() - before; got != hiddenFilesSpliceBatch {
		t.Fatalf("added %d entries, want one splice batch %d", got, hiddenFilesSpliceBatch)
	}
	if s.hidden.filesSpliceAt != hiddenFilesSpliceBatch {
		t.Fatalf("filesSpliceAt = %d, want %d", s.hidden.filesSpliceAt, hiddenFilesSpliceBatch)
	}
}

func newHiddenWorkTestSession(root string) *session {
	return &session{
		gen:            1,
		opts:           StartOpts{DisplayRoot: root, IncludeHidden: true},
		idx:            newIndex(),
		walks:          make(map[string]*walkHandle),
		completedRoots: make(map[string]struct{}),
		hidden:         *newHiddenState(),
		coord:          &Coordinator{notify: func(Event) {}},
	}
}

func hiddenTestItoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	return string(buf[pos:])
}
