package dialog

import (
	"sync"
	"testing"

	"github.com/paranoidi/paras-commander/internal/diskusage"
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

func TestFsCtxAttachesIndexSinkOnlyForPanelsThatFollowIt(t *testing.T) {
	t.Parallel()
	s := newSnapshotHarness(t, true)
	sink := &indexLog{}
	s.h.indexSink = sink // no disk-usage engine: the index sink must still be attached
	change := fschange.Change{Kind: fschange.Removed, Path: "/orchard/harbor.txt"}

	fschange.Report(s.h.fsCtx(), change)
	if len(sink.got) != 0 {
		t.Fatalf("a normal local panel reported %v to the index", sink.got)
	}

	s.h.host.ActivePanel().WithIndex = true
	fschange.Report(s.h.fsCtx(), change)
	if len(sink.got) != 1 {
		t.Fatalf("a browse-with panel reported %v to the index, want one change", sink.got)
	}

	// With both sinks and no index sink wired, the context still works.
	s.h.diskUsage = diskusage.New()
	s.h.indexSink = nil
	fschange.Report(s.h.fsCtx(), change)
	if len(sink.got) != 1 {
		t.Fatal("a cleared index sink still received changes")
	}
}
