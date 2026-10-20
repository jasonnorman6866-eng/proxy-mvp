package meter

import (
	"sync"

	"golang.org/x/time/rate"
)

type Meter struct {
	mu         sync.Mutex
	bytes      int64
	sessions   int64
	limiter    *rate.Limiter
}

func New(limitPerMinute, burst int) *Meter {
	if limitPerMinute <= 0 {
		limitPerMinute = 60
	}
	if burst <= 0 {
		burst = 30
	}
	return &Meter{
		limiter: rate.NewLimiter(rate.Limit(limitPerMinute/60.0), burst),
	}
}

func (m *Meter) Allow() bool {
	return m.limiter.Allow()
}

func (m *Meter) RecordBytes(n int64) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bytes += n
	m.sessions++
}

func (m *Meter) Snapshot() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"bytes_transferred": m.bytes,
		"sessions_count":    m.sessions,
	}
}
