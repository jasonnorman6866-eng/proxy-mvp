package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type Gateway struct {
	username string
	password string
	allowedHosts map[string]bool
}

func newGateway() *Gateway {
	return &Gateway{
		username: env("PROXY_USER", "dev"),
		password: env("PROXY_PASSWORD", "change-me"),
		allowedHosts: map[string]bool{
			"example.com": true,
		},
	}
}

func TestHealthEndpoint(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/", g)
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health body: %s", w.Body.String())
	}
}

func TestCONNECTAuthorizedDestination(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 for relay not enabled, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "relay not enabled") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestCONNECTRejectsUnauthorizedHost(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodConnect, "https://www.google.com:443", nil)
	req.Host = "www.google.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCONNECTRejectsIncorrectAuth(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:wrong")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)

	if w.Code != http.StatusProxyAuthRequired {
		t.Fatalf("expected 407, got %d", w.Code)
	}
}

func main() {
	g := newGateway()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
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
		return
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "invalid destination", http.StatusBadRequest)
		return
	}

	if port != "443" {
		http.Error(w, "destination port denied", http.StatusForbidden)
		return
	}

	if !g.allowedHosts[strings.ToLower(host)] {
		http.Error(w, "destination denied", http.StatusForbidden)
		log.Printf("audit action=deny host=%q port=%q remote=%q", host, port, r.RemoteAddr)
		return
	}

	http.Error(w, "destination authorized; relay not enabled in gateway v0.1", http.StatusNotImplemented)
	log.Printf("audit action=authorized host=%q port=%q remote=%q", host, port, r.RemoteAddr)
}

func (g *Gateway) authenticate(r *http.Request) bool {
	value := r.Header.Get("Proxy-Authorization")
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
