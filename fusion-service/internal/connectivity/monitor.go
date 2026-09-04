// Package connectivity tracks whether fusion-service should treat itself as
// "offline" — in which case writes go to the local SQLite queue only
// (see internal/localqueue) instead of straight to Postgres.
package connectivity

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Monitor combines a real reachability check (periodic Postgres ping) with
// a manual override (POST /debug/set-offline), so offline behavior can be
// demoed/tested on demand without actually taking Postgres down.
type Monitor struct {
	pool *pgxpool.Pool

	forcedOffline atomic.Bool
	pingFailed    atomic.Bool

	becameOnline chan struct{}
}

func NewMonitor(pool *pgxpool.Pool) *Monitor {
	return &Monitor{pool: pool, becameOnline: make(chan struct{}, 1)}
}

// IsOffline reports whether writes should currently be queued locally
// instead of going straight to Postgres.
func (m *Monitor) IsOffline() bool {
	return m.forcedOffline.Load() || m.pingFailed.Load()
}

// SetForcedOffline is the manual override behind POST /debug/set-offline.
func (m *Monitor) SetForcedOffline(offline bool) {
	wasOffline := m.IsOffline()
	m.forcedOffline.Store(offline)
	if wasOffline && !m.IsOffline() {
		m.signalOnline()
	}
}

// Run periodically pings Postgres and updates real connectivity state
// until ctx is canceled, signaling BecameOnline() on every offline->online
// transition (whether caused by the ping recovering or by a forced-offline
// override being cleared while the ping was already healthy).
func (m *Monitor) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			wasOffline := m.IsOffline()

			pingCtx, cancel := context.WithTimeout(ctx, interval/2)
			err := m.pool.Ping(pingCtx)
			cancel()
			m.pingFailed.Store(err != nil)

			if wasOffline && !m.IsOffline() {
				m.signalOnline()
			}
		}
	}
}

func (m *Monitor) signalOnline() {
	select {
	case m.becameOnline <- struct{}{}:
	default:
	}
}

// BecameOnline receives a signal on every offline->online transition, so a
// drain loop can react immediately instead of waiting for its next poll.
func (m *Monitor) BecameOnline() <-chan struct{} {
	return m.becameOnline
}
