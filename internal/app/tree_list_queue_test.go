package app

import (
	"slices"
	"sync"
	"testing"
	"time"
)

func TestTreeListQueuePriorityOrderAndConcurrency(t *testing.T) {
	q := &treeListQueue{workers: 1}
	gate := make(chan struct{})
	started := make(chan struct{})
	var mu sync.Mutex
	var order []string
	var wg sync.WaitGroup
	rec := func(name string) func() {
		wg.Add(1)
		return func() {
			defer wg.Done()
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}
	wg.Add(1)
	q.push(prioOffscreen, func() { defer wg.Done(); close(started); <-gate })
	<-started
	q.push(prioOffscreen, rec("offscreen"))
	q.push(prioVisible, rec("visible"))
	q.push(prioCaret, rec("caret"))
	q.push(prioUser, rec("user"))
	q.mu.Lock()
	if q.active != 1 {
		t.Errorf("active = %d, want 1 (peak <= workers)", q.active)
	}
	q.mu.Unlock()
	close(gate)
	wg.Wait()
	if want := []string{"user", "caret", "visible", "offscreen"}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		q.mu.Lock()
		n := q.active
		q.mu.Unlock()
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("active = %d after drain, want 0", n)
		}
		time.Sleep(time.Millisecond)
	}
}
