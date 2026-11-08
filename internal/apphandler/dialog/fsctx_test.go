package dialog

import (
	"sync"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fschange"
)

type indexLog struct {
	mu  sync.Mutex
	got []fschange.Change
}

func (l *indexLog) Apply(b []fschange.Change) {
	l.mu.Lock()
	l.got = append(l.got, b...)
	l.mu.Unlock()
}

func TestFsCtxReportsToSharedAndIndexSinks(t *testing.T) {
	t.Parallel()
	s := newSnapshotHarness(t, true)
	sink, shared := &indexLog{}, &indexLog{}
	s.h.indexSink, s.h.fsSink = sink, shared
	change := fschange.Change{Kind: fschange.Removed, Path: "/orchard/harbor.txt"}

	fschange.Report(s.h.fsCtx(), change) // a normal local panel
	if len(sink.got) != 1 || len(shared.got) != 1 {
		t.Fatalf("index sink %v, shared sink %v; want one change each", sink.got, shared.got)
	}

	s.h.indexSink = nil
	fschange.Report(s.h.fsCtx(), change)
	if len(sink.got) != 1 || len(shared.got) != 2 {
		t.Fatalf("index sink %v, shared sink %v; a cleared index sink must not receive it", sink.got, shared.got)
	}
}
