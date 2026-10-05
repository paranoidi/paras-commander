package app

import (
	"sync"

	"github.com/paranoidi/paras-commander/internal/config"
)

// Tree listing priorities, highest first.
type treeListPrio int

const (
	prioUser      treeListPrio = iota // expand / expand-all / navigation re-entry
	prioCaret                         // refresh of dirs adjacent to the cursor
	prioVisible                       // refresh of on-screen dirs
	prioOffscreen                     // refresh of everything else
	treeListPrios
)

// treeListQueue bounds tree directory listing concurrency app-wide (both panels share it) and
// serves higher priorities first, FIFO within a priority. Workers start on demand and exit when
// the queue drains, so no goroutine idles. Jobs are always invoked, even if their work was
// cancelled: each job checks its own ctx first and returns without I/O, which keeps callers'
// WaitGroup accounting simple. The zero value is usable (workers defaults to
// config.DefaultTreeListWorkers).
type treeListQueue struct {
	mu      sync.Mutex
	workers int // 0 = config.DefaultTreeListWorkers
	active  int
	queues  [treeListPrios][]func()
}

func (q *treeListQueue) push(prio treeListPrio, fn func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queues[prio] = append(q.queues[prio], fn)
	limit := q.workers
	if limit <= 0 {
		limit = config.DefaultTreeListWorkers
	}
	if q.active < limit {
		q.active++
		go q.work()
	}
}

func (q *treeListQueue) work() {
	for {
		q.mu.Lock()
		var fn func()
		for p := range q.queues {
			if len(q.queues[p]) > 0 {
				fn = q.queues[p][0]
				q.queues[p][0] = nil
				q.queues[p] = q.queues[p][1:]
				break
			}
		}
		if fn == nil {
			q.active--
			q.mu.Unlock()
			return
		}
		q.mu.Unlock()
		fn()
	}
}
