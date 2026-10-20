package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"proxy-mvp/internal/audit"
	"proxy-mvp/internal/auth"
	"proxy-mvp/internal/meter"
	"proxy-mvp/internal/policy"
)

type Server struct {
	AuthEnabled           bool
	Username              string
	Password              string
	Policy                *policy.Policy
	Meter                 *meter.Meter
	Logger                *audit.Logger
	EgressEnabled         bool
	MaxConnectionsPerUser int
	MaxConnectionsPerHandle int
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	if r.URL.Path == "/health" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAuthorized(r, s.AuthEnabled, s.Username, s.Password) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="proxy-mvp"`)
		http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	if !s.EgressEnabled {
		http.Error(w, "Egress disabled", http.StatusServiceUnavailable)
		return
	}

	host, port, err := parseTarget(r.Host)
	if err != nil {
		http.Error(w, "Invalid CONNECT target", http.StatusBadRequest)
		return
	}
	if !s.Policy.Allow(port) {
		http.Error(w, "Destination port not allowed", http.StatusForbidden)
		return
	}

	userKey := r.RemoteAddr
	if !s.Meter.Allow(userKey, s.MaxConnectionsPerUser) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	target := net.JoinHostPort(host, strconv.Itoa(port))
	upstream, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		log.Printf("upstream dial failed for %s: %v", target, err)
		http.Error(w, "Upstream connection failed", http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		_ = upstream.Close()
		http.Error(w, "Could not hijack client connection", http.StatusInternalServerError)
		return
	}

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		_ = upstream.Close()
		return
	}

	start := time.Now()
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = clientConn.Close()
			_ = upstream.Close()
		})
	}

	var bytesUp int64
	var bytesDown int64
	go func() {
		defer closeBoth()
		buf := make([]byte, 32*1024)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				bytesUp += int64(n)
				if _, werr := upstream.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer closeBoth()
		buf := make([]byte, 32*1024)
		for {
			n, err := upstream.Read(buf)
			if n > 0 {
				bytesDown += int64(n)
				if _, werr := clientConn.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		time.Sleep(time.Second)
		if !isConnOpen(clientConn) || !isConnOpen(upstream) {
			closeBoth()
		}
	}()

	s.Logger.Log(map[string]any{
		"event":        "connect_established",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"user":         userKey,
		"destination_host": host,
		"destination_port": port,
		"bytes_uploaded":   bytesUp,
		"bytes_downloaded": bytesDown,
		"connection_duration_ms": time.Since(start).Milliseconds(),
		"exit_node": "authorized-test-egress",
		"result": "ok",
	})

	for {
		if !isConnOpen(clientConn) || !isConnOpen(upstream) {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	closeBoth()
	_ = s.Meter.Record(userKey, bytesUp, bytesDown, time.Since(start), host, port)
}

func parseTarget(raw string) (string, int, error) {
	if raw == "" {
		return "", 0, fmt.Errorf("missing target")
	}
	if strings.Contains(raw, ":") {
		host, portStr, err := net.SplitHostPort(raw)
		if err == nil {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return "", 0, err
			}
			return host, port, nil
		}
	}
	return "", 0, fmt.Errorf("target must be host:port")
}

func isConnOpen(conn net.Conn) bool {
	if conn == nil {
		return false
	}
	return true
}
