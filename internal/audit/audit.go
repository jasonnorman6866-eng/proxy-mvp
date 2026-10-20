package audit

import (
	"encoding/json"
	"log"
	"sync"
)

type Logger struct {
	enabled bool
	mu      sync.Mutex
}

func New(enabled bool) *Logger {
	return &Logger{enabled: enabled}
}

func (l *Logger) Log(fields map[string]any) {
	if !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	b, err := json.Marshal(fields)
	if err != nil {
		return
	}
	log.Printf("%s", string(b))
}
