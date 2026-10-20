package audit

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

type Logger struct {
	enabled bool
	mu      sync.Mutex
}

func New(enabled bool) *Logger {
	return &Logger{enabled: enabled}
}

func (l *Logger) Log(event string, fields map[string]any) {
	if !l.enabled {
		return
	}
	payload := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339), "event": event}
	for k, v := range fields {
		payload[k] = v
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	log.Printf("%s", string(b))
}
