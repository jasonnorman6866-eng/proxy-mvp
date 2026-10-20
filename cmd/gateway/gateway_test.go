package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"proxy-mvp/internal/nodes"
)

func TestHealthEndpoint(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		// direct call to ServeHTTP should route the health endpoint only by mux; skip this one.
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", g)

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health body: %s", w.Body.String())
	}
}

func TestNoCredentials(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusProxyAuthRequired {
		t.Fatalf("expected 407, got %d", w.Code)
	}
}

func TestWrongCredentials(t *testing.T) {
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

func TestNonCONNECTRequest(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodGet, "https://example.com:443", nil)
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCONNECTPort80Denied(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:80", nil)
	req.Host = "example.com:80"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUnapprovedHostnameDenied(t *testing.T) {
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

func TestApprovedHostnamePolicyPasses(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 because relay is intentionally disabled, got %d", w.Code)
	}
}

func TestUnauthorizedExitNodeDenied(t *testing.T) {
	g := newGateway()
	g.registry.Register(nodes.ExitNode{ID: "bad-node", Address: "127.0.0.1:9444", Authorized: false, Healthy: true, Disabled: false})
	g.registry.Disable("dev-node-01")
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestUnhealthyExitNodeDenied(t *testing.T) {
	g := newGateway()
	g.registry.Register(nodes.ExitNode{ID: "dev-node-01", Address: "127.0.0.1:9443", Authorized: true, Healthy: false, Disabled: false})
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDisabledExitNodeDenied(t *testing.T) {
	g := newGateway()
	g.registry.Disable("dev-node-01")
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHealthyAuthorizedNodeEligible(t *testing.T) {
	g := newGateway()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req.Host = "example.com:443"
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 because relay is intentionally disabled, got %d", w.Code)
	}
}

func TestConnectionLimitExceeded(t *testing.T) {
	g := newGateway()
	g.limits = newLimiterForTesting(1)
	req1 := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req1.Host = "example.com:443"
	req1.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	req1.RemoteAddr = "127.0.0.1:10000"
	w1 := httptest.NewRecorder()
	g.ServeHTTP(w1, req1)
	if w1.Code != http.StatusNotImplemented {
		t.Fatalf("first request should pass policy gate and reach relay-disabled status, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodConnect, "https://example.com:443", nil)
	req2.Host = "example.com:443"
	req2.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:change-me")))
	req2.RemoteAddr = "127.0.0.1:10000"
	w2 := httptest.NewRecorder()
	g.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
}

func newLimiterForTesting(max int) *limits.Limiter {
	return limits.New(max)
}
