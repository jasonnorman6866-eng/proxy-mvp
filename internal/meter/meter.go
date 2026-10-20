package meter

import (
	"sync"

	"golang.org/x/time/rate"
)

type Meter struct {
	mu      sync.Mutex
	bytes   int64
	limiter *rate.Limiter
}

func New(rateLimitPerMinute, burst int) *Meter {
	if rateLimitPerMinute <= 0 {
		rateLimitPerMinute = 60
	}
	if burst <= 0 {
		burst = 30
	}
	return &Meter{limiter: rate.NewLimiter(rate.Limit(rateLimitPerMinute/60.0), burst)}
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
}

func (m *Meter) Bytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bytes
}
