package audit

import (
	"encoding/json"
	"log"
	"sync"
)

type Logger struct {
	enabled     bool
	logPayloads bool
	mu          sync.Mutex
}

func New(enabled bool, logPayloads bool) *Logger {
	return &Logger{enabled: enabled, logPayloads: logPayloads}
}

func (l *Logger) Log(fields map[string]any) {
	if !l.enabled {
		return
	}
	if !l.logPayloads {
		delete(fields, "request_body")
		delete(fields, "response_body")
		delete(fields, "cookies")
		delete(fields, "tls")
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	log.Printf("%s", string(b))
}
