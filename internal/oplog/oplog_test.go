package oplog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/paranoidi/paras-commander/internal/search"
)

func label(i int) string { return fmt.Sprintf("meadow/lantern-%05d.txt", i) }

func newTestLog(t *testing.T, memoryRows int) (*Log, *Session) {
	t.Helper()
	sess := NewSession(t.TempDir())
	l := New("job-harbor", Config{MemoryRows: memoryRows, Session: sess})
	t.Cleanup(l.Close)
	return l, sess
}

func labels(t *testing.T, l *Log, nums []uint32) []string {
	t.Helper()
	recs, err := l.ReadRecords(nums)
	if err != nil {
		t.Fatalf("ReadRecords: %v", err)
	}
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.Label
	}
	return out
}

func TestAppendCountsAndNewestFirstView(t *testing.T) {
	l, _ := newTestLog(t, 0)
	l.Append(Record{Result: Done, Label: "orchard/acorn.txt"})
	l.Append(Record{Result: Skipped, Label: "orchard/bramble.txt", Reason: "exists"})
	l.Append(Record{Result: Failed, Label: "orchard/clover.txt", Reason: "permission denied"})
	st := l.Stats()
	if st.Done != 1 || st.Skipped != 1 || st.Failed != 1 || st.Dropped != 0 {
		t.Fatalf("stats = %+v", st)
	}
	if got := labels(t, l, l.ViewNums(MaskAll, NoSource, 0, 10)); fmt.Sprint(got) != "[orchard/clover.txt orchard/bramble.txt orchard/acorn.txt]" {
		t.Fatalf("all view = %v", got)
	}
	if got := labels(t, l, l.ViewNums(1<<Skipped, NoSource, 0, 10)); fmt.Sprint(got) != "[orchard/bramble.txt]" {
		t.Fatalf("skipped view = %v", got)
	}
	if n := l.ViewLen(1<<Done|1<<Failed, NoSource); n != 2 {
		t.Fatalf("done+failed len = %d", n)
	}
}

func TestRetrySupersedesFailure(t *testing.T) {
	l, _ := newTestLog(t, 0)
	l.Append(Record{Result: Failed, Label: "harbor/willow.bin", Reason: "input/output error"})
	l.Append(Record{Result: Done, Label: "harbor/copper.bin"})
	l.Append(Record{Result: Done, Label: "harbor/willow.bin"}) // retry succeeded
	st := l.Stats()
	if st.Failed != 0 || st.Done != 2 {
		t.Fatalf("stats = %+v, want the failure superseded", st)
	}
	if got := labels(t, l, l.ViewNums(MaskAll, NoSource, 0, 10)); fmt.Sprint(got) != "[harbor/willow.bin harbor/copper.bin]" {
		t.Fatalf("all view = %v, want the failed row hidden", got)
	}
	if n := l.ViewLen(1<<Failed, NoSource); n != 0 {
		t.Fatalf("failed view len = %d", n)
	}
}

func TestSpillKeepsNewestInMemoryAndReadsOldFromDisk(t *testing.T) {
	const rows = 1000
	l, sess := newTestLog(t, rows)
	total := 5*rows + 17
	for i := range total {
		res := Done
		if i%250 == 0 {
			res = Failed
		}
		l.Append(Record{Result: res, Label: label(i), Reason: "reason-" + strconv.Itoa(i)})
	}
	path := filepath.Join(sess.Dir(), "job-harbor.oplog")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("spill file: %v", err)
	}
	newest := l.ViewNums(MaskAll, NoSource, 0, rows)
	if _, ok := l.RecordsInMemory(newest); !ok {
		t.Fatal("the newest MemoryRows rows must stay in memory")
	}
	oldest := l.ViewNums(MaskAll, NoSource, total-3, 3)
	if _, ok := l.RecordsInMemory(oldest); ok {
		t.Fatal("the oldest rows should only be on disk")
	}
	if got := labels(t, l, oldest); fmt.Sprint(got) != fmt.Sprint([]string{label(2), label(1), label(0)}) {
		t.Fatalf("oldest rows = %v", got)
	}
	recs, err := l.ReadRecords([]uint32{3001, 0, 1251})
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].Label != label(3001) || recs[1].Reason != "reason-0" || recs[2].Result != Done || recs[1].Result != Failed {
		t.Fatalf("scattered read = %+v", recs)
	}
	failed := l.ViewNums(1<<Failed, NoSource, 0, 100)
	if len(failed) != total/250+1 {
		t.Fatalf("failed rows = %d", len(failed))
	}
	l.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Close left the spill file: %v", err)
	}
}

func TestSpillFailureDropsOldestRows(t *testing.T) {
	l := New("job-quarry", Config{MemoryRows: 1000})
	defer l.Close()
	for i := range 2500 {
		l.Append(Record{Result: Done, Label: label(i)})
	}
	st := l.Stats()
	if st.Dropped == 0 || st.Done != 2500 {
		t.Fatalf("stats = %+v, want dropped rows and the true done count", st)
	}
	if n := l.ViewLen(MaskAll, NoSource); n != 2500-st.Dropped {
		t.Fatalf("view len = %d, want %d", n, 2500-st.Dropped)
	}
	if _, err := l.ReadRecords([]uint32{0}); err == nil {
		t.Fatal("a dropped row must not be readable")
	}
}

func TestScanAcrossDiskAndMemory(t *testing.T) {
	l, _ := newTestLog(t, 1000)
	for i := range 4000 {
		name := label(i)
		if i%997 == 0 {
			name = fmt.Sprintf("thistle/beacon-%d.log", i)
		}
		l.Append(Record{Result: Done, Label: name})
	}
	var got []uint32
	upto, err := l.Scan(context.Background(), 0, MaskAll, NoSource, search.Parse("beacon"), search.Options{CaseInsensitive: true}, func(b []uint32) { got = append(got, b...) })
	if err != nil || upto != 4000 {
		t.Fatalf("Scan = %d, %v", upto, err)
	}
	if fmt.Sprint(got) != "[0 997 1994 2991 3988]" {
		t.Fatalf("matches = %v", got)
	}
	l.Append(Record{Result: Done, Label: "thistle/beacon-new.log"})
	got = nil
	if upto, _ = l.Scan(context.Background(), upto, MaskAll, NoSource, search.Parse("beacon"), search.Options{}, func(b []uint32) { got = append(got, b...) }); upto != 4001 || fmt.Sprint(got) != "[4000]" {
		t.Fatalf("incremental scan = %d %v", upto, got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Scan(ctx, 0, MaskAll, NoSource, search.Parse("beacon"), search.Options{}, func([]uint32) {}); err == nil {
		t.Fatal("canceled scan returned no error")
	}
}

func TestSweepStaleRemovesDeadSessions(t *testing.T) {
	root := t.TempDir()
	sess := NewSession(root)
	if err := os.MkdirAll(sess.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	dead := filepath.Join(root, "pc", "jobs", "2147483646-1700000000")
	if err := os.MkdirAll(dead, 0o700); err != nil {
		t.Fatal(err)
	}
	sess.SweepStale()
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatalf("dead session kept: %v", err)
	}
	if _, err := os.Stat(sess.Dir()); err != nil {
		t.Fatalf("own session removed: %v", err)
	}
	if err := sess.Remove(); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentAppendAndRead(t *testing.T) {
	l, _ := newTestLog(t, 1000)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 20000 {
			l.Append(Record{Result: Result(i % 3), Label: label(i)})
		}
	}()
	for {
		select {
		case <-done:
			if n := l.ViewLen(MaskAll, NoSource); n != 20000 {
				t.Fatalf("len = %d", n)
			}
			return
		default:
		}
		nums := l.ViewNums(1<<Done|1<<Skipped, NoSource, l.ViewLen(1<<Done|1<<Skipped, NoSource)/2, 20)
		if _, err := l.ReadRecords(nums); err != nil {
			t.Fatalf("ReadRecords: %v", err)
		}
		_, _ = l.Scan(context.Background(), 0, MaskAll, NoSource, search.Parse("lantern-0001"), search.Options{}, func([]uint32) {})
	}
}

func TestPerSourceViewsAndStats(t *testing.T) {
	sess := NewSession(t.TempDir())
	l := New("job-ridge", Config{MemoryRows: 100, Session: sess, Sources: 3})
	t.Cleanup(l.Close)
	for i := range 300 {
		src := int32(i % 3)
		res := Done
		if i%50 == 0 {
			res = Failed
		}
		l.Append(Record{Result: res, Label: label(i), Source: src})
	}
	l.Append(Record{Result: Done, Label: "ledger/undo-step.txt", Source: NoSource})
	l.Append(Record{Result: Done, Label: label(0), Source: 0}) // retry supersedes source 0's failure

	per := l.SourceStats()
	if len(per) != 3 {
		t.Fatalf("SourceStats = %+v", per)
	}
	// Failures at i = 0, 150 (source 0), 50, 200 (source 2), 100, 250 (source 1); i=0 retried.
	if per[0].Failed != 1 || per[0].Done != 99 || per[1].Failed != 2 || per[2].Failed != 2 {
		t.Fatalf("per-source stats = %+v", per)
	}
	if n := l.ViewLen(MaskAll, 1); n != 100 {
		t.Fatalf("source 1 view len = %d", n)
	}
	if n := l.ViewLen(MaskAll, NoSource); n != 302-1 {
		t.Fatalf("all view len = %d", n)
	}
	if n := l.ViewLen(MaskAll, 7); n != 0 {
		t.Fatalf("unknown source view len = %d", n)
	}
	oldest := l.ViewNums(MaskAll, 2, 99, 1)
	recs, err := l.ReadRecords(oldest)
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].Label != label(2) || recs[0].Source != 2 {
		t.Fatalf("oldest source 2 record (from the spill file) = %+v", recs[0])
	}
	var got []uint32
	if _, err := l.Scan(context.Background(), 0, 1<<Failed, 1, search.Parse("lantern"), search.Options{}, func(b []uint32) { got = append(got, b...) }); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[100 250]" {
		t.Fatalf("source 1 failed scan = %v", got)
	}
}

func TestSingleSourceLogHasNoSourceStats(t *testing.T) {
	l, _ := newTestLog(t, 0)
	l.Append(Record{Result: Done, Label: "orchard/acorn.txt"})
	if per := l.SourceStats(); per != nil {
		t.Fatalf("SourceStats = %v", per)
	}
}
