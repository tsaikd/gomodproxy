package store

import (
	"context"
	"errors"
	"sync"

	"github.com/sixt/gomodproxy/pkg/vcs"
)

type memory struct {
	sync.Mutex
	cache []Snapshot
}

// Memory creates an in-memory cache.
func Memory() Store { return &memory{} }

func (m *memory) Put(ctx context.Context, snapshot Snapshot) error {
	m.Lock()
	defer m.Unlock()
	for _, item := range m.cache {
		if item.Module == snapshot.Module && item.Version == snapshot.Version {
			return nil
		}
	}
	m.cache = append(m.cache, snapshot)
	return nil
}

func (m *memory) Get(ctx context.Context, module string, version vcs.Version) (Snapshot, error) {
	m.Lock()
	defer m.Unlock()
	for _, snapshot := range m.cache {
		if snapshot.Module == module && snapshot.Version == version {
			return snapshot, nil
		}
	}
	return Snapshot{}, errors.New("not found")
}

func (m *memory) Close() error { return nil }
