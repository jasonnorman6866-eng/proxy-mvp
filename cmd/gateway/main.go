package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"proxy-mvp/internal/audit"
	"proxy-mvp/internal/limits"
	"proxy-mvp/internal/nodes"
	"proxy-mvp/internal/policy"
)

type Gateway struct {
	username string
	password string

	policy  *policy.Policy
	registry *nodes.Registry
	limits  *limits.Limiter
	logger  *audit.Logger
}

func newGateway() *Gateway {
	pol := policy.New([]string{"example.com"}, []int{443})
	reg := nodes.NewRegistry([]nodes.ExitNode{
		{ID: "dev-node-01", Address: "127.0.0.1:9443", Authorized: true, Healthy: true, Disabled: false},
	})
	return &Gateway{
		username: env("PROXY_USER", "dev"),
		password: env("PROXY_PASSWORD", "change-me"),
		policy:   pol,
		registry: reg,
		limits:   limits.New(1),
		logger:   audit.New(true),
	}
}

func main() {
	g := newGateway()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/", g)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	log.Println("gateway listening on :8080")
	log.Fatal(server.ListenAndServe())
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !g.authenticate(r) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="proxy-mvp"`)
		http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}

	if r.Method != http.MethodConnect {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "non_connect_request"})
		return
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "invalid destination", http.StatusBadRequest)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "invalid_target", "host": r.Host})
		return
	}

	if port != "443" {
		http.Error(w, "destination port denied", http.StatusForbidden)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "destination_port_not_allowed", "host": host, "port": port})
		return
	}

	if !g.policy.Allow(strings.ToLower(host), 443) {
		http.Error(w, "destination not allowed", http.StatusForbidden)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "destination_not_allowed", "host": host, "port": port})
		return
	}

	userKey := r.RemoteAddr
	if !g.limits.Allow(userKey) {
		http.Error(w, "connection limit exceeded", http.StatusTooManyRequests)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "connection_limit_exceeded", "remote_addr": userKey})
		return
	}

	node, err := g.registry.GetAvailable()
	if err != nil {
		http.Error(w, "no eligible exit node available", http.StatusServiceUnavailable)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": err.Error(), "remote_addr": userKey})
		return
	}

	g.logger.Log(map[string]any{
		"action":     "connection_eligible",
		"host":       host,
		"port":       port,
		"node_id":    node.ID,
		"node_addr":  node.Address,
		"remote_addr": userKey,
	})

	// v0.2 gate: node eligibility passes, but actual relay remains intentionally disabled.
	http.Error(w, "authorized exit node selected; relay disabled until v0.3", http.StatusNotImplemented)
	g.logger.Log(map[string]any{"action": "relay_blocked", "host": host, "port": port, "node_id": node.ID, "remote_addr": userKey})
}

func (g *Gateway) authenticate(r *http.Request) bool {
	value := r.Header.Get("Proxy-Authorization")
	if value == "" {
		value = r.Header.Get("Authorization")
	}
	const prefix = "Basic "
	if !strings.HasPrefix(value, prefix) {
		return false
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return false
	}

	user, pass, ok := strings.Cut(string(raw), ":")
	if !ok {
		return false
	}

	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(g.username))
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(g.password))
	return userOK == 1 && passOK == 1
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func mustJSON(t *testing.T, data any) string {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return string(b)
}

func ExampleGateway() {
	fmt.Println("gateway v0.2")
	// Output: gateway v0.2
}
