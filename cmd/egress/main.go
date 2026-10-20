package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	listenAddr := env("EGRESS_LISTEN_ADDR", ":9443")
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen egress: %v", err)
	}
	defer ln.Close()
	log.Printf("egress node listening on %s", listenAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("read CONNECT request: %v", err)
		return
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "CONNECT ") {
		_, _ = fmt.Fprintf(conn, "ERROR invalid request\r\n")
		return
	}
	target := strings.TrimPrefix(line, "CONNECT ")
	if !strings.Contains(target, ":") {
		_, _ = fmt.Fprintf(conn, "ERROR invalid target\r\n")
		return
	}
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		_, _ = fmt.Fprintf(conn, "ERROR invalid target\r\n")
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		_, _ = fmt.Fprintf(conn, "ERROR invalid port\r\n")
		return
	}
	if host == "" || port <= 0 {
		_, _ = fmt.Fprintf(conn, "ERROR invalid target\r\n")
		return
	}
	destAddr := net.JoinHostPort(host, strconv.Itoa(port))
	upstream, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		_, _ = fmt.Fprintf(conn, "ERROR dial failed: %v\r\n", err)
		log.Printf("dial target %s: %v", destAddr, err)
		return
	}
	if _, err := fmt.Fprintf(conn, "OK\r\n"); err != nil {
		_ = upstream.Close()
		return
	}
	log.Printf("relay established %s -> %s", conn.RemoteAddr().String(), destAddr)
	go relay(conn, upstream)
	relay(upstream, conn)
}

func relay(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	_, _ = io.Copy(dst, src)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
