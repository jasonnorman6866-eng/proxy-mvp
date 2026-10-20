package limits

import (
	"sync"

	"golang.org/x/time/rate"
)

type Meter struct {
	mu          sync.Mutex
	bytes       int64
	connections int64
	rl          *rate.Limiter
}

func NewMeter(limit, burst int) *Meter {
	if limit <= 0 {
		limit = 60
	}
	if burst <= 0 {
		burst = 30
	}
	return &Meter{
		rl: rate.NewLimiter(rate.Limit(limit), burst),
	}
}

func (m *Meter) Allow() bool {
	return m.rl.Allow()
}

func (m *Meter) RecordBytes(n int64) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bytes += n
	m.connections++
}

func (m *Meter) Bytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bytes
}

func (m *Meter) Connections() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connections
}
