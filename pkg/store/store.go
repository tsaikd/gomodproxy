package store

import (
	"context"
	"time"

	"github.com/sixt/gomodproxy/pkg/vcs"
)

type Store interface {
	Put(ctx context.Context, snapshot Snapshot) error
	Get(ctx context.Context, module string, version vcs.Version) (Snapshot, error)
	Close() error
}

type Snapshot struct {
	Module    string
	Version   vcs.Version
	Timestamp time.Time
	Data      []byte
}

func (s Snapshot) Key() string {
	return s.Module + "@" + string(s.Version)
}
