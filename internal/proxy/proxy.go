package proxy

import (
	"encoding/json"
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
	"proxy-mvp/internal/config"
	"proxy-mvp/internal/meter"
	"proxy-mvp/internal/policy"
)

type Proxy struct {
	cfg    *config.Config
	policy *policy.Policy
	meter  *meter.Meter
	audit  *audit.Logger
}

func New(cfg *config.Config) *Proxy {
	return &Proxy{
		cfg:    cfg,
		policy: policy.New(cfg.Policy.AllowedHosts, cfg.Policy.AllowedPorts),
		meter:  meter.New(cfg.Meter.RateLimitPerMinute, cfg.Meter.Burst),
		audit:  audit.New(cfg.Audit.Enabled),
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == p.cfg.Gateway.HealthPath && r.Method == http.MethodGet {
		p.handleHealth(w, r)
		return
	}
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{
		"status": "ok",
		"gateway": map[string]any{
			"listen": p.cfg.Gateway.Listen,
		},
		"exit_node": map[string]any{
			"host":        p.cfg.ExitNode.Host,
			"port":        p.cfg.ExitNode.Port,
			"label":       p.cfg.ExitNode.Label,
			"authorized":  p.cfg.ExitNode.Authorized,
			"healthy":     p.cfg.ExitNode.Healthy,
			"kill_switch": p.cfg.ExitNode.KillSwitch,
		},
		"bytes_transferred": p.meter.Bytes(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode health payload: %v", err)
	}
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAuthorized(r, p.cfg.Gateway.Username, p.cfg.Gateway.Password) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="proxy-mvp"`)
		http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	if p.cfg.ExitNode.KillSwitch {
		http.Error(w, "Proxy kill switch engaged", http.StatusServiceUnavailable)
		return
	}
	if !p.cfg.ExitNode.Authorized || !p.cfg.ExitNode.Healthy {
		http.Error(w, "Exit node not healthy or not authorized", http.StatusServiceUnavailable)
		return
	}

	targetHost, targetPort, err := parseTarget(r.Host)
	if err != nil {
		http.Error(w, "invalid target", http.StatusBadRequest)
		return
	}
	if !p.policy.Allow(targetHost, targetPort) {
		http.Error(w, "Destination not allowed by policy", http.StatusForbidden)
		return
	}
	if !p.meter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	upstreamAddr := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))
	upstreamConn, err := net.DialTimeout("tcp", upstreamAddr, 10*time.Second)
	if err != nil {
		log.Printf("dial upstream %s: %v", upstreamAddr, err)
		http.Error(w, "Upstream connection failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstreamConn.Close()
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		_ = upstreamConn.Close()
		log.Printf("hijack client: %v", err)
		http.Error(w, "Could not hijack client connection", http.StatusInternalServerError)
		return
	}
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		_ = upstreamConn.Close()
		return
	}

	p.audit.Log("connect_established", map[string]any{
		"client":         r.RemoteAddr,
		"target_host":    targetHost,
		"target_port":    targetPort,
		"node_label":     p.cfg.ExitNode.Label,
		"node_authorized": p.cfg.ExitNode.Authorized,
		"node_healthy":   p.cfg.ExitNode.Healthy,
	})

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = clientConn.Close()
			_ = upstreamConn.Close()
		})
	}

	go func() {
		defer closeBoth()
		_, _ = io.Copy(&countingWriter{meter: p.meter, target: clientConn}, upstreamConn)
	}()
	go func() {
		defer closeBoth()
		_, _ = io.Copy(&countingWriter{meter: p.meter, target: upstreamConn}, clientConn)
	}()
}

type countingWriter struct {
	meter  *meter.Meter
	target io.Writer
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.target.Write(p)
	if n > 0 {
		cw.meter.RecordBytes(int64(n))
	}
	return n, err
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
