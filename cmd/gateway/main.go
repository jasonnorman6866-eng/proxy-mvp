package main

import (
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"proxy-mvp/internal/audit"
	"proxy-mvp/internal/auth"
	"proxy-mvp/internal/limits"
	"proxy-mvp/internal/nodes"
	"proxy-mvp/internal/policy"
	"proxy-mvp/internal/relay"
)

type GatewayConfig struct {
	Listen      string
	HealthPath  string
	Username    string
	Password    string
	AllowedHost string
	AllowedPort int
	MaxConns    int
	ReadTimeout time.Duration
	WriteTimeout time.Duration
}

type Gateway struct {
	cfg     GatewayConfig
	policy  *policy.Policy
	registry *nodes.Registry
	limits  *limits.Limiter
	logger  *audit.Logger
}

func newGateway() *Gateway {
	cfg := GatewayConfig{
		Listen:      ":8080",
		HealthPath:  "/health",
		Username:    env("PROXY_USER", "dev"),
		Password:    env("PROXY_PASSWORD", "secret"),
		AllowedHost: "127.0.0.1",
		AllowedPort: 9444,
		MaxConns:    5,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	p := policy.New([]string{cfg.AllowedHost, "localhost"}, []int{cfg.AllowedPort})
	reg := nodes.NewRegistry([]nodes.ExitNode{
		{ID: "dev-node-01", Address: "127.0.0.1:9443", Authorized: true, Healthy: true, Disabled: false},
	})

	return &Gateway{
		cfg:     cfg,
		policy:  p,
		registry: reg,
		limits:  limits.New(cfg.MaxConns),
		logger:  audit.New(true),
	}
}

func main() {
	g := newGateway()

	mux := http.NewServeMux()
	mux.HandleFunc(g.cfg.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", g)

	server := &http.Server{
		Addr:              g.cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:      30 * time.Second,
		MaxHeaderBytes:   16 << 10,
	}

	log.Printf("gateway listening on %s", g.cfg.Listen)
	log.Fatal(server.ListenAndServe())
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAuthorized(r, g.cfg.Username, g.cfg.Password) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="proxy-mvp"`)
		http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}

	if r.Method != http.MethodConnect {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "non_connect_request", "remote": r.RemoteAddr})
		return
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "invalid target", http.StatusBadRequest)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "invalid_target", "target": r.Host, "remote": r.RemoteAddr})
		return
	}

	if !g.policy.Allow(strings.ToLower(host), portToInt(port)) {
		http.Error(w, "destination not allowed", http.StatusForbidden)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "destination_not_allowed", "host": host, "port": port, "remote": r.RemoteAddr})
		return
	}

	if !g.limits.Allow(r.RemoteAddr) {
		http.Error(w, "connection limit exceeded", http.StatusTooManyRequests)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "limit_exceeded", "remote": r.RemoteAddr})
		return
	}

	node, err := g.registry.GetAvailable()
	if err != nil {
		http.Error(w, "no available exit node", http.StatusServiceUnavailable)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": err.Error(), "remote": r.RemoteAddr})
		return
	}

	g.logger.Log(map[string]any{"action": "connect_attempt", "host": host, "port": port, "node": node.ID, "remote": r.RemoteAddr})

	upstream, err := net.DialTimeout("tcp", node.Address, 5*time.Second)
	if err != nil {
		log.Printf("dial exit node %s: %v", node.Address, err)
		http.Error(w, "exit node unavailable", http.StatusBadGateway)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "node_unreachable", "node": node.ID, "remote": r.RemoteAddr})
		return
	}
	defer upstream.Close()

	if _, err := upstream.Write([]byte("CONNECT " + host + ":" + port + "\r\n")); err != nil {
		log.Printf("write CONNECT to exit node: %v", err)
		http.Error(w, "exit node rejected tunnel", http.StatusBadGateway)
		return
	}

	reply := make([]byte, 64)
	upstream.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := upstream.Read(reply)
	if err != nil || !strings.Contains(string(reply[:n]), "OK") {
		log.Printf("exit node handshake failed: %v payload=%q", err, string(reply[:n]))
		http.Error(w, "exit node rejected tunnel", http.StatusBadGateway)
		return
	}
	upstream.SetReadDeadline(time.Time{})

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Printf("hijack client: %v", err)
		http.Error(w, "Could not hijack client connection", http.StatusInternalServerError)
		return
	}
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		return
	}

	g.logger.Log(map[string]any{"action": "tunnel_established", "host": host, "port": port, "node": node.ID, "remote": r.RemoteAddr})
	go relay(clientConn, upstream)
	relay(upstream, clientConn)
}

func relay(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") || err == io.EOF {
				return
			}
			return
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func portToInt(port string) int {
	v, err := strconv.Atoi(port)
	if err != nil {
		return -1
	}
	return v
}
