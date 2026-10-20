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
	"proxy-mvp/internal/config"
	"proxy-mvp/internal/meter"
	"proxy-mvp/internal/policy"
)

type Handler struct {
	cfg    *config.Config
	policy *policy.Policy
	meter  *meter.Meter
	audit  *audit.Logger
}

func New(cfg *config.Config) *Handler {
	return &Handler{
		cfg:    cfg,
		policy: policy.New(cfg.Policy.AllowedHosts, cfg.Policy.AllowedPorts),
		meter:  meter.New(cfg.Meter.LimitPerMinute, cfg.Meter.Burst),
		audit:  audit.New(cfg.Audit.Enabled),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == h.cfg.Gateway.HealthPath && r.Method == http.MethodGet {
		h.handleHealth(w, r)
		return
	}

	if r.Method == http.MethodConnect {
		h.handleCONNECT(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"status": "ok",
		"gateway": map[string]any{
			"listen": h.cfg.Gateway.Listen,
		},
		"exit_node": map[string]any{
			"host":       h.cfg.ExitNode.Host,
			"port":       h.cfg.ExitNode.Port,
			"authorized": h.cfg.ExitNode.Authorized,
			"healthy":    h.cfg.ExitNode.Healthy,
			"kill_switch": h.cfg.ExitNode.KillSwitch,
		},
		"usage": h.meter.Snapshot(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = jsonWrite(w, status)
}

func (h *Handler) handleCONNECT(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAuthorized(r, h.cfg.Gateway.Username, h.cfg.Gateway.Password) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="proxy-mvp"`)
		http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
		return
	}

	if h.cfg.ExitNode.KillSwitch {
		http.Error(w, "Proxy kill switch engaged", http.StatusServiceUnavailable)
		return
	}
	if !h.cfg.ExitNode.Authorized || !h.cfg.ExitNode.Healthy {
		http.Error(w, "Exit node not healthy or not authorized", http.StatusServiceUnavailable)
		return
	}

	targetHost, targetPort, err := parseTarget(r.Host)
	if err != nil {
		http.Error(w, "invalid target", http.StatusBadRequest)
		return
	}

	if !h.policy.Allow(targetHost, targetPort) {
		http.Error(w, "Destination not allowed by policy", http.StatusForbidden)
		return
	}

	if !h.meter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	dialTarget := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))
	upstream, err := net.DialTimeout("tcp", dialTarget, 10*time.Second)
	if err != nil {
		log.Printf("dial upstream %s: %v", dialTarget, err)
		http.Error(w, "Upstream connection failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		log.Printf("hijack client: %v", err)
		http.Error(w, "Could not hijack client connection", http.StatusInternalServerError)
		return
	}

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		_ = upstream.Close()
		return
	}

	h.audit.Log("connect_established", map[string]any{
		"client":       r.RemoteAddr,
		"target_host":  targetHost,
		"target_port":  targetPort,
		"node_label":   h.cfg.ExitNode.Label,
		"node_authorized": h.cfg.ExitNode.Authorized,
		"node_healthy": h.cfg.ExitNode.Healthy,
	})

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = clientConn.Close()
			_ = upstream.Close()
		})
	}

	go func() {
		defer closeBoth()
		_, _ = io.Copy(clientConn, upstream)
	}()
	go func() {
		defer closeBoth()
		_, _ = io.Copy(upstream, clientConn)
	}()
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

func jsonWrite(w http.ResponseWriter, payload map[string]any) error {
	w.Header().Set("Content-Type", "application/json")
	_, err := fmt.Fprintf(w, "{\n")
	if err != nil {
		return err
	}
	// small JSON envelope for health output without external packages
	if len(payload) == 0 {
		_, err = fmt.Fprintf(w, "}\n")
		return err
	}
	first := true
	for k, v := range payload {
		if !first {
			_, _ = fmt.Fprintf(w, ",\n")
		}
		first = false
		b, _ := jsonMarshal(v)
		_, _ = fmt.Fprintf(w, "  \"%s\": %s", k, b)
	}
	_, err = fmt.Fprintf(w, "\n}\n")
	return err
}

func jsonMarshal(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x), nil
	case int, int64, int32, float32, float64, bool:
		return fmt.Sprintf("%v", x), nil
	case map[string]any:
		b, err := jsonMarshalMap(x)
		return b, err
	case []any:
		b, err := jsonMarshalSlice(x)
		return b, err
	default:
		return strconv.Quote(fmt.Sprintf("%v", x)), nil
	}
}

func jsonMarshalMap(m map[string]any) (string, error) {
	var out strings.Builder
	out.WriteString("{")
	first := true
	for k, v := range m {
		if !first {
			out.WriteString(", ")
		}
		first = false
		b, err := jsonMarshal(v)
		if err != nil {
			return "{}", err
		}
		out.WriteString(strconv.Quote(k))
		out.WriteString(": ")
		out.WriteString(b)
	}
	out.WriteString("}")
	return out.String(), nil
}

func jsonMarshalSlice(a []any) (string, error) {
	var out strings.Builder
	out.WriteString("[")
	for i, v := range a {
		if i > 0 {
			out.WriteString(", ")
		}
		b, err := jsonMarshal(v)
		if err != nil {
			return "[]", err
		}
		out.WriteString(b)
	}
	out.WriteString("]")
	return out.String(), nil
}
