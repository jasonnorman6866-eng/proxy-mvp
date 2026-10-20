package meter

import (
	"sync"
	"time"
)

type Meter struct {
	mu      sync.Mutex
	counts  map[string]int
	records map[string]SessionRecord
}

type SessionRecord struct {
	User         string
	Host         string
	Port         int
	BytesUp      int64
	BytesDown    int64
	DurationMs   int64
	EndedAt      time.Time
}

func New() *Meter {
	return &Meter{
		counts:  make(map[string]int),
		records: make(map[string]SessionRecord),
	}
}

func (m *Meter) Allow(user string, max int) bool {
	if max <= 0 {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[user]++
	return m.counts[user] <= max
}

func (m *Meter) Record(user string, bytesUp, bytesDown int64, duration time.Duration, host string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[user] = SessionRecord{
		User:       user,
		Host:       host,
		Port:       port,
		BytesUp:    bytesUp,
		BytesDown:  bytesDown,
		DurationMs: duration.Milliseconds(),
		EndedAt:    time.Now().UTC(),
	}
	return nil
}
