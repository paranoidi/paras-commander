//go:build linux

package dirwatch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"
)

const waitTimeout = 5 * time.Second

// recorder collects onChange calls.
type recorder struct {
	mu   sync.Mutex
	dirs []string
}

func (r *recorder) add(d string) {
	r.mu.Lock()
	r.dirs = append(r.dirs, d)
	r.mu.Unlock()
}

func (r *recorder) count(d string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, x := range r.dirs {
		if x == d {
			n++
		}
	}
	return n
}

// waitNotified polls until dir was reported at least once.
func (r *recorder) waitNotified(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for r.count(dir) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no notification for %s", dir)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// watchedFixture starts a watcher over a fresh dir (after setup populated it) and waits until
// the watch is armed (the worker applies Set asynchronously).
func watchedFixture(t *testing.T, setup func(dir string)) (*Watcher, *recorder, string) {
	t.Helper()
	rec := &recorder{}
	w := New(rec.add)
	t.Cleanup(w.Close)
	dir := t.TempDir()
	if setup != nil {
		setup(dir)
	}
	w.Set([]string{dir})
	waitArmed(t, w, dir, true)
	return w, rec, dir
}

func waitArmed(t *testing.T, w *Watcher, dir string, want bool) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		w.mu.Lock()
		_, ok := w.dirWd[dir]
		w.mu.Unlock()
		if ok == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("watch on %s armed=%v, want %v", dir, ok, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestNotifiesOnEachChangeKind(t *testing.T) {
	// Each case's setup runs before the watch is armed, so only act can produce the event.
	cases := map[string]struct {
		setup func(dir string)
		act   func(dir string)
	}{
		"create": {nil, func(dir string) { mustWrite(t, filepath.Join(dir, "meadow.txt")) }},
		"delete": {
			func(dir string) { mustWrite(t, filepath.Join(dir, "willow.txt")) },
			func(dir string) { mustNil(t, os.Remove(filepath.Join(dir, "willow.txt"))) },
		},
		"rename in": {nil, func(dir string) {
			src := filepath.Join(t.TempDir(), "pebble.txt")
			mustWrite(t, src)
			mustNil(t, os.Rename(src, filepath.Join(dir, "pebble.txt")))
		}},
		"rename out": {
			func(dir string) { mustWrite(t, filepath.Join(dir, "river.txt")) },
			func(dir string) {
				mustNil(t, os.Rename(filepath.Join(dir, "river.txt"), filepath.Join(t.TempDir(), "river.txt")))
			},
		},
		"chmod": {
			func(dir string) { mustWrite(t, filepath.Join(dir, "cobble.txt")) },
			func(dir string) { mustNil(t, os.Chmod(filepath.Join(dir, "cobble.txt"), 0o600)) },
		},
		"write and close": {
			func(dir string) { mustWrite(t, filepath.Join(dir, "lantern.txt")) },
			func(dir string) {
				f, err := os.OpenFile(filepath.Join(dir, "lantern.txt"), os.O_WRONLY|os.O_APPEND, 0)
				mustNil(t, err)
				_, _ = f.WriteString("more")
				mustNil(t, f.Close())
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, rec, dir := watchedFixture(t, tc.setup)
			if n := rec.count(dir); n != 0 {
				t.Fatalf("%d notifications before the action", n)
			}
			tc.act(dir)
			rec.waitNotified(t, dir)
		})
	}
}

// Writes to a growing file are reported once, by IN_CLOSE_WRITE; IN_MODIFY and the read-side
// events would flood the queue or make a listing trigger itself.
func TestMaskExcludesNoisyEvents(t *testing.T) {
	for _, bit := range []uint32{syscall.IN_MODIFY, syscall.IN_ACCESS, syscall.IN_OPEN, syscall.IN_CLOSE_NOWRITE} {
		if watchMask&bit != 0 {
			t.Fatalf("watch mask includes noisy event %#x", bit)
		}
	}
}

func TestSetRemovalStopsNotifications(t *testing.T) {
	w, rec, dir := watchedFixture(t, nil)
	keep := t.TempDir()
	w.Set([]string{keep})
	waitArmed(t, w, dir, false)
	waitArmed(t, w, keep, true)
	mustWrite(t, filepath.Join(dir, "gone.txt"))
	mustWrite(t, filepath.Join(keep, "kept.txt"))
	rec.waitNotified(t, keep)
	if n := rec.count(dir); n != 0 {
		t.Fatalf("unwatched dir reported %d times", n)
	}
}

func TestWatchedDirRemovedNotifiesAndWatcherSurvives(t *testing.T) {
	w, rec, _ := watchedFixture(t, nil)
	parent := t.TempDir()
	doomed := filepath.Join(parent, "doomed")
	if err := os.Mkdir(doomed, 0o755); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	w.Set([]string{doomed, other})
	waitArmed(t, w, doomed, true)
	waitArmed(t, w, other, true)
	if err := os.Remove(doomed); err != nil {
		t.Fatal(err)
	}
	rec.waitNotified(t, doomed)
	waitArmed(t, w, doomed, false) // IN_IGNORED dropped the mapping
	mustWrite(t, filepath.Join(other, "later.txt"))
	rec.waitNotified(t, other)

	// Recreated and set again: watched once more.
	if err := os.Mkdir(doomed, 0o755); err != nil {
		t.Fatal(err)
	}
	w.Set([]string{doomed, other})
	waitArmed(t, w, doomed, true)
}

func TestSetHarmlessPaths(t *testing.T) {
	w, rec, dir := watchedFixture(t, nil)
	file := filepath.Join(dir, "plain.txt")
	mustWrite(t, file)
	w.Set([]string{dir, file, filepath.Join(dir, "nowhere"), ""})
	waitArmed(t, w, dir, true)
	// Let the worker finish the batch, then prove the watcher still works.
	mustWrite(t, filepath.Join(dir, "after.txt"))
	rec.waitNotified(t, dir)
	w.mu.Lock()
	n := len(w.dirWd)
	w.mu.Unlock()
	if n != 1 {
		t.Fatalf("watching %d dirs, want only the real directory", n)
	}
}

func TestCloseIsIdempotentAndPrompt(t *testing.T) {
	w := New(func(string) {})
	w.Set([]string{t.TempDir()})
	done := make(chan struct{})
	go func() {
		w.Close()
		w.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(waitTimeout):
		t.Fatal("Close did not return")
	}
	// The reader goroutine must exit (Read unblocked by the close).
	w.Set([]string{t.TempDir()})
}

func TestOverflowNotifiesEveryWatchedDir(t *testing.T) {
	rec := &recorder{}
	w := New(rec.add)
	t.Cleanup(w.Close)
	a, b := t.TempDir(), t.TempDir()
	w.Set([]string{a, b})
	waitArmed(t, w, a, true)
	waitArmed(t, w, b, true)
	w.handle(-1, syscall.IN_Q_OVERFLOW)
	if rec.count(a) != 1 || rec.count(b) != 1 {
		t.Fatalf("overflow reported a=%d b=%d, want 1 each", rec.count(a), rec.count(b))
	}
}

func TestUnmountDropsWatchAndNotifies(t *testing.T) {
	w, rec, dir := watchedFixture(t, nil)
	w.mu.Lock()
	wd := w.dirWd[dir]
	w.mu.Unlock()
	w.handle(wd, syscall.IN_UNMOUNT)
	if rec.count(dir) != 1 {
		t.Fatalf("unmount reported %d times, want 1", rec.count(dir))
	}
	waitArmed(t, w, dir, false)
}

func TestParseEventsSeveralRecords(t *testing.T) {
	rec := func(wd int32, mask uint32, name string) []byte {
		padded := len(name)
		if padded > 0 {
			padded = (len(name) + 1 + 15) &^ 15 // NUL-terminated and padded, as the kernel does
		}
		b := make([]byte, syscall.SizeofInotifyEvent+padded)
		binary.NativeEndian.PutUint32(b[0:], uint32(wd))
		binary.NativeEndian.PutUint32(b[4:], mask)
		binary.NativeEndian.PutUint32(b[12:], uint32(padded))
		copy(b[syscall.SizeofInotifyEvent:], name)
		return b
	}
	var buf []byte
	buf = append(buf, rec(3, syscall.IN_CREATE, "quiet-harbor.txt")...)
	buf = append(buf, rec(4, syscall.IN_DELETE_SELF, "")...) // empty name field
	buf = append(buf, rec(-1, syscall.IN_Q_OVERFLOW, "")...)
	buf = append(buf, rec(5, syscall.IN_ATTRIB, "x")...)

	type ev struct {
		wd   int32
		mask uint32
	}
	var got []ev
	parseEvents(buf, func(wd int32, mask uint32) { got = append(got, ev{wd, mask}) })
	want := []ev{{3, syscall.IN_CREATE}, {4, syscall.IN_DELETE_SELF}, {-1, syscall.IN_Q_OVERFLOW}, {5, syscall.IN_ATTRIB}}
	if !slices.Equal(got, want) {
		t.Fatalf("parsed %v, want %v", got, want)
	}

	// A record cut off mid-name and a stray partial header are ignored, earlier ones still count.
	got = nil
	cut := append(slices.Clone(buf), rec(6, syscall.IN_CREATE, "truncated-name")[:20]...)
	parseEvents(cut, func(wd int32, mask uint32) { got = append(got, ev{wd, mask}) })
	if !slices.Equal(got, want) {
		t.Fatalf("with truncated tail parsed %v, want %v", got, want)
	}
	parseEvents(buf[:7], func(int32, uint32) { t.Fatal("callback on partial header") })
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustNil(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
