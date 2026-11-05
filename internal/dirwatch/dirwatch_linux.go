//go:build linux

// Package dirwatch reports changes inside a set of directories (inotify on Linux, a no-op
// elsewhere). It only says "this directory changed": callers re-list it themselves. Every
// failure (inotify unavailable, watch limit, permissions, vanished directories) just leaves that
// directory unwatched, so callers keep a periodic refresh as the safety net.
package dirwatch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// watchMask lists the events that change what a directory listing shows (names, types, sizes,
// mtimes, modes). IN_MODIFY is left out on purpose: it fires for every write(2) of a growing
// file and would flood the queue; IN_CLOSE_WRITE reports the final size and mtime once. IN_ACCESS
// and IN_OPEN are left out too, since listing the directory would otherwise trigger itself.
// IN_ONLYDIR makes a non-directory path fail instead of watching a file.
const watchMask = syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO |
	syscall.IN_CLOSE_WRITE | syscall.IN_ATTRIB | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF | syscall.IN_ONLYDIR

// Watcher watches directories and calls onChange with the directory path (as passed to Set,
// cleaned) when something in it changed. onChange runs on the reader goroutine and must not block
// for long. The zero set of watches is valid; a Watcher whose inotify setup failed is inert.
type Watcher struct {
	onChange func(dir string)
	f        *os.File // inotify instance; nil when unavailable
	fd       int

	mu      sync.Mutex
	closed  bool
	desired []string
	wdDir   map[int32]string // watch descriptor -> directory
	dirWd   map[string]int32
	kick    chan struct{} // wakes the worker after Set
	done    chan struct{}
}

// New starts a watcher. It never fails: without inotify the result simply never reports.
func New(onChange func(dir string)) *Watcher {
	w := &Watcher{
		onChange: onChange,
		wdDir:    map[int32]string{},
		dirWd:    map[string]int32{},
		kick:     make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return w
	}
	// A non-blocking fd goes through the runtime poller, so closing f unblocks a pending Read.
	w.fd, w.f = fd, os.NewFile(uintptr(fd), "inotify")
	go w.readLoop()
	go w.workLoop()
	return w
}

// Set replaces the watched directories with dirs. It does no I/O and never blocks: a worker
// goroutine adds and removes the watches (inotify_add_watch resolves the path and can hang on a
// stalled network mount). A newer set replaces one the worker has not applied yet.
func (w *Watcher) Set(dirs []string) {
	want := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d != "" { // Clean("") would be the working directory
			want = append(want, filepath.Clean(d))
		}
	}
	w.mu.Lock()
	if w.closed || w.f == nil {
		w.mu.Unlock()
		return
	}
	w.desired = want
	w.mu.Unlock()
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// Close stops the watcher. It is idempotent and does not wait for the worker, which may be stuck
// in a syscall on a dead mount; it exits once that syscall returns.
func (w *Watcher) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	close(w.done)
	w.mu.Unlock()
	if w.f != nil {
		// A worker still inside a syscall may later use a recycled fd number; inotify_add_watch
		// and inotify_rm_watch reject any fd that is not an inotify instance (EINVAL).
		_ = w.f.Close()
	}
}

func (w *Watcher) workLoop() {
	for {
		select {
		case <-w.done:
			return
		case <-w.kick:
		}
		w.apply()
	}
}

// apply diffs the desired set against the live watches. Syscalls run without the lock so the
// reader is never held up by a slow add.
func (w *Watcher) apply() {
	w.mu.Lock()
	want := make(map[string]bool, len(w.desired))
	var add []string
	for _, d := range w.desired {
		want[d] = true
		if _, ok := w.dirWd[d]; !ok {
			add = append(add, d)
		}
	}
	var drop []int32
	for d, wd := range w.dirWd {
		if !want[d] {
			drop = append(drop, wd)
		}
	}
	for _, wd := range drop {
		// Forget before removing, so the IN_IGNORED the kernel then sends is not reported.
		delete(w.dirWd, w.wdDir[wd])
		delete(w.wdDir, wd)
	}
	w.mu.Unlock()
	for _, wd := range drop {
		_, _ = syscall.InotifyRmWatch(w.fd, uint32(wd))
	}
	for _, d := range add {
		select {
		case <-w.done:
			return
		default:
		}
		wd, err := syscall.InotifyAddWatch(w.fd, d, watchMask)
		if err != nil {
			continue // ENOENT, EACCES, ENOTDIR, ENOSPC...: periodic refresh covers it
		}
		w.mu.Lock()
		if w.closed {
			w.mu.Unlock()
			return
		}
		if old, dup := w.dirWd[d]; dup && old != int32(wd) {
			delete(w.wdDir, old)
		}
		// The kernel returns the same wd for a second path to the same inode: last one wins.
		if prev, ok := w.wdDir[int32(wd)]; ok && prev != d {
			delete(w.dirWd, prev)
		}
		w.wdDir[int32(wd)] = d
		w.dirWd[d] = int32(wd)
		w.mu.Unlock()
	}
}

func (w *Watcher) readLoop() {
	buf := make([]byte, 64*1024)
	for {
		n, err := w.f.Read(buf)
		if err != nil {
			return // closed, or the instance is unusable: nothing more will arrive
		}
		parseEvents(buf[:n], w.handle)
	}
}

// handle reports one decoded event. wd -1 is the queue overflow: events were lost, so everything
// watched counts as changed.
func (w *Watcher) handle(wd int32, mask uint32) {
	var dirs []string
	w.mu.Lock()
	switch {
	case w.closed:
	case mask&syscall.IN_Q_OVERFLOW != 0:
		for d := range w.dirWd {
			dirs = append(dirs, d)
		}
	default:
		dir, ok := w.wdDir[wd]
		if !ok {
			break
		}
		dirs = append(dirs, dir)
		// The kernel drops the watch itself on IN_IGNORED/IN_UNMOUNT; a moved directory keeps
		// reporting under a stale path, so release it and let the next Set re-add it by path.
		if mask&(syscall.IN_IGNORED|syscall.IN_UNMOUNT|syscall.IN_MOVE_SELF) != 0 {
			delete(w.wdDir, wd)
			delete(w.dirWd, dir)
			if mask&syscall.IN_MOVE_SELF != 0 {
				_, _ = syscall.InotifyRmWatch(w.fd, uint32(wd))
			}
		}
	}
	w.mu.Unlock()
	for _, d := range dirs {
		w.onChange(d)
	}
}

// parseEvents decodes the variable-length inotify_event records in buf (a fixed 16-byte header
// followed by len bytes of NUL-padded name, possibly zero) and calls fn for each. A truncated
// trailing record is ignored.
func parseEvents(buf []byte, fn func(wd int32, mask uint32)) {
	const hdr = syscall.SizeofInotifyEvent
	for len(buf) >= hdr {
		wd := int32(binary.NativeEndian.Uint32(buf[0:4]))
		mask := binary.NativeEndian.Uint32(buf[4:8])
		nameLen := int(binary.NativeEndian.Uint32(buf[12:16]))
		if hdr+nameLen > len(buf) {
			return
		}
		fn(wd, mask)
		buf = buf[hdr+nameLen:]
	}
}
