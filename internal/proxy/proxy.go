package proxy

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"proxy-mvp/internal/auth"
	"proxy-mvp/internal/config"
	"proxy-mvp/internal/limits"
)

type Proxy struct {
	cfg   config.Config
	meter *limits.Meter
}

func New(cfg config.Config) *Proxy {
	return &Proxy{
		cfg:   cfg,
		meter: limits.NewMeter(cfg.RateLimit, cfg.RateBurst),
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "This proxy only supports CONNECT", http.StatusMethodNotAllowed)
		return
	}

	if !auth.IsAuthorized(r, p.cfg.BasicUser, p.cfg.BasicPassword) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="proxy-mvp"`)
		http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
		return
	}

	if p.cfg.KillSwitch {
		http.Error(w, "Proxy kill switch engaged", http.StatusServiceUnavailable)
		return
	}

	targetHost, targetPort, err := parseTarget(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !p.cfg.IsAllowedDestination(targetHost, targetPort) {
		http.Error(w, "Destination not allowed by policy", http.StatusForbidden)
		return
	}

	if !p.meter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	nodeAddr := net.JoinHostPort(p.cfg.ExitNodeHost, strconv.Itoa(p.cfg.ExitNodePort))
	nodeConn, err := net.DialTimeout("tcp", nodeAddr, 10*time.Second)
	if err != nil {
		log.Printf("dial exit node: %v", err)
		http.Error(w, "Exit node unavailable", http.StatusBadGateway)
		return
	}

	if _, err := fmt.Fprintf(nodeConn, "CONNECT %s:%d\r\n", targetHost, targetPort); err != nil {
		log.Printf("write CONNECT to exit node: %v", err)
		nodeConn.Close()
		http.Error(w, "Could not establish exit tunnel", http.StatusBadGateway)
		return
	}

	reader := bufio.NewReader(nodeConn)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("read exit node handshake: %v", err)
		nodeConn.Close()
		http.Error(w, "Exit node rejected the tunnel", http.StatusBadGateway)
		return
	}

	if strings.TrimSpace(line) != "OK" {
		log.Printf("exit node handshake rejected: %q", strings.TrimSpace(line))
		nodeConn.Close()
		http.Error(w, "Exit node rejected the tunnel", http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Printf("hijack client conn: %v", err)
		nodeConn.Close()
		http.Error(w, "Could not hijack client connection", http.StatusInternalServerError)
		return
	}

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		log.Printf("write 200 to client: %v", err)
		clientConn.Close()
		nodeConn.Close()
		return
	}

	log.Printf("tunnel established: client=%s target=%s:%d via node=%s", r.RemoteAddr, targetHost, targetPort, nodeAddr)

	go relay(clientConn, nodeConn, p.meter)
	relay(nodeConn, clientConn, p.meter)
}

func parseTarget(r *http.Request) (string, int, error) {
	target := r.Host
	if target == "" {
		target = r.URL.Host
	}
	if target == "" {
		return "", 0, fmt.Errorf("missing target")
	}

	var host string
	var port int
	var err error

	if strings.Contains(target, ":") {
		host, _, err = net.SplitHostPort(target)
		if err != nil {
			return "", 0, fmt.Errorf("target %q is not valid host:port", target)
		}
		port, err = strconv.Atoi(strings.TrimPrefix(target[strings.LastIndex(target, ":")+1:], ""))
		if err != nil {
			return "", 0, fmt.Errorf("invalid port in target %q", target)
		}
	} else {
		host = target
		port = 443
	}

	if host == "" {
		return "", 0, fmt.Errorf("missing host in target %q", target)
	}

	return host, port, nil
}

func relay(dst, src net.Conn, meter *limits.Meter) {
	defer dst.Close()
	defer src.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
			meter.RecordBytes(int64(n))
		}
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			return
		}
	}
}
