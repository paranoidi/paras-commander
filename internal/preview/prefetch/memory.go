package prefetch

import (
	"container/list"
	"sync"
)

type memEntry struct {
	key   string
	png   []byte
	meta  string
	bytes int64
}

// memoryLRU is a byte-budget LRU of PNG rasters.
type memoryLRU struct {
	mu     sync.Mutex
	budget int64
	used   int64
	order  *list.List // front = most recent; Value *memEntry
	byKey  map[string]*list.Element
}

func newMemoryLRU(budgetBytes int64) *memoryLRU {
	if budgetBytes < 1 {
		budgetBytes = 1
	}
	return &memoryLRU{
		budget: budgetBytes,
		order:  list.New(),
		byKey:  make(map[string]*list.Element),
	}
}

func (m *memoryLRU) get(key string) (png []byte, meta string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	el, ok := m.byKey[key]
	if !ok {
		return nil, "", false
	}
	m.order.MoveToFront(el)
	e := el.Value.(*memEntry)
	out := make([]byte, len(e.png))
	copy(out, e.png)
	return out, e.meta, true
}

func (m *memoryLRU) put(key string, png []byte, meta string) {
	if len(png) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.byKey[key]; ok {
		e := el.Value.(*memEntry)
		m.used -= e.bytes
		m.order.Remove(el)
		delete(m.byKey, key)
	}
	size := int64(len(png))
	for m.used+size > m.budget && m.order.Len() > 0 {
		back := m.order.Back()
		old := back.Value.(*memEntry)
		m.used -= old.bytes
		m.order.Remove(back)
		delete(m.byKey, old.key)
	}
	if size > m.budget {
		// Single entry larger than budget: keep it alone (ponytail: avoid empty cache forever).
		m.used = 0
		m.order.Init()
		clear(m.byKey)
	}
	cp := make([]byte, len(png))
	copy(cp, png)
	e := &memEntry{key: key, png: cp, meta: meta, bytes: int64(len(cp))}
	el := m.order.PushFront(e)
	m.byKey[key] = el
	m.used += e.bytes
	for m.used > m.budget && m.order.Len() > 1 {
		back := m.order.Back()
		old := back.Value.(*memEntry)
		m.used -= old.bytes
		m.order.Remove(back)
		delete(m.byKey, old.key)
	}
}

func (m *memoryLRU) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.byKey[key]
	return ok
}
