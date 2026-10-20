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
			log.Printf("accept err: %v", err)
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
		log.Printf("read CONNECT: %v", err)
		return
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "CONNECT ") {
		_, _ = fmt.Fprintf(conn, "ERROR invalid request\r\n")
		return
	}
	target := strings.TrimPrefix(line, "CONNECT ")
	if target == "" {
		_, _ = fmt.Fprintf(conn, "ERROR invalid target\r\n")
		return
	}
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
		log.Printf("dial %s failed: %v", destAddr, err)
		return
	}
	if _, err := fmt.Fprintf(conn, "OK\r\n"); err != nil {
		_ = upstream.Close()
		return
	}
	log.Printf("tunnel %s -> %s", conn.RemoteAddr().String(), destAddr)
	go relay(conn, upstream)
	relay(upstream, conn)
}

func relay(dst, src net.Conn) { defer dst.Close(); defer src.Close(); _, _ = io.Copy(dst, src) }

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
