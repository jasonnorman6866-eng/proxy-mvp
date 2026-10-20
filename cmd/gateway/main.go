package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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
	pol := policy.New([]string{"example.com", "localhost", "127.0.0.1"}, []int{443, 9444})
	reg := nodes.NewRegistry([]nodes.ExitNode{
		{ID: "dev-node-01", Address: "127.0.0.1:9443", Authorized: true, Healthy: true, Disabled: false},
	})
	return &Gateway{
		username: env("PROXY_USER", "dev"),
		password: env("PROXY_PASSWORD", "secret"),
		policy:   pol,
		registry: reg,
		limits:   limits.New(5),
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
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "non_connect_request", "remote_addr": r.RemoteAddr})
		return
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "invalid destination", http.StatusBadRequest)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "invalid_target", "host": r.Host, "remote_addr": r.RemoteAddr})
		return
	}

	if !g.policy.Allow(strings.ToLower(host), portToInt(port)) {
		http.Error(w, "destination not allowed", http.StatusForbidden)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "destination_not_allowed", "host": host, "port": port, "remote_addr": r.RemoteAddr})
		return
	}

	if !g.limits.Allow(r.RemoteAddr) {
		http.Error(w, "connection limit exceeded", http.StatusTooManyRequests)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "connection_limit_exceeded", "remote_addr": r.RemoteAddr})
		return
	}

	node, err := g.registry.GetAvailable()
	if err != nil {
		http.Error(w, "no eligible exit node available", http.StatusServiceUnavailable)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": err.Error(), "remote_addr": r.RemoteAddr})
		return
	}

	nodeConn, err := net.DialTimeout("tcp", node.Address, 10*time.Second)
	if err != nil {
		log.Printf("connect to exit node %s: %v", node.Address, err)
		http.Error(w, "exit node unavailable", http.StatusBadGateway)
		g.logger.Log(map[string]any{"action": "connection_denied", "reason": "exit_node_unreachable", "node_id": node.ID, "remote_addr": r.RemoteAddr})
		return
	}
	defer nodeConn.Close()

	if _, err := fmt.Fprintf(nodeConn, "CONNECT %s:%s\r\n", host, port); err != nil {
		log.Printf("write CONNECT to exit node: %v", err)
		http.Error(w, "exit node rejected tunnel", http.StatusBadGateway)
		return
	}

	buf := make([]byte, 64)
	if err := nodeConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("set read deadline: %v", err)
	}
	if n, readErr := nodeConn.Read(buf); readErr != nil || !strings.Contains(string(buf[:n]), "OK") {
		log.Printf("exit node handshake failed: n=%d err=%v payload=%q", n, readErr, string(buf[:n]))
		http.Error(w, "exit node rejected tunnel", http.StatusBadGateway)
		return
	}
	if err := nodeConn.SetReadDeadline(time.Time{}); err != nil {
		log.Printf("clear read deadline: %v", err)
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Printf("hijack: %v", err)
		http.Error(w, "Could not hijack client connection", http.StatusInternalServerError)
		return
	}
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		return
	}

	g.logger.Log(map[string]any{"action": "tunnel_established", "host": host, "port": port, "node_id": node.ID, "remote_addr": r.RemoteAddr})
	go relay(clientConn, nodeConn)
	relay(nodeConn, clientConn)
}

func relay(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = copyLoop(dst, src)
	}()
	go func() {
		defer wg.Done()
		_, _ = copyLoop(src, dst)
	}()
	wg.Wait()
}

func copyLoop(dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			m, werr := dst.Write(buf[:n])
			written += int64(m)
			if werr != nil {
				return written, werr
			}
		}
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return written, nil
			}
			return written, err
		}
	}
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

func portToInt(port string) int {
	value, err := strconv.Atoi(port)
	if err != nil {
		return -1
	}
	return value
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
