package ingestion

import (
	"sync"
	"time"
)

type DedupeStore interface {
	Seen(key string, now time.Time, window time.Duration) bool
}

type MemoryDedupeStore struct {
	mu sync.Mutex
	seen map[string]time.Time
}

func NewMemoryDedupeStore() *MemoryDedupeStore {
	return &MemoryDedupeStore{seen: map[string]time.Time{}}
}

func (m *MemoryDedupeStore) Seen(key string, now time.Time, window time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if key == "" {
		return false
	}
	if ts, ok := m.seen[key]; ok && now.Sub(ts) <= window {
		return true
	}
	m.seen[key] = now
	return false
}
