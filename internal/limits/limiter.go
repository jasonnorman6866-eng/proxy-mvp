package limits

import "sync"

type Limiter struct {
	mu     sync.Mutex
	max    int
	counts map[string]int
}

func New(max int) *Limiter {
	if max <= 0 {
		max = 1
	}
	return &Limiter{max: max, counts: make(map[string]int)}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counts[key]++
	return l.counts[key] <= l.max
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counts, key)
}
