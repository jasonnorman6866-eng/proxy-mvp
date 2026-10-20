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
	listenAddr := getEnv("NODE_LISTEN_ADDR", ":9001")
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	log.Printf("exit node listening on %s", listenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleTunnel(conn)
	}
}

func handleTunnel(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("read tunnel request: %v", err)
		return
	}

	target := strings.TrimSpace(line)
	if !strings.HasPrefix(target, "CONNECT ") {
		_, _ = fmt.Fprintf(conn, "ERROR bad-request\r\n")
		return
	}

	target = strings.TrimPrefix(target, "CONNECT ")
	target = strings.TrimSpace(target)

	if target == "" {
		_, _ = fmt.Fprintf(conn, "ERROR missing-target\r\n")
		return
	}

	host, port, err := parseTarget(target)
	if err != nil {
		_, _ = fmt.Fprintf(conn, "ERROR %s\r\n", err.Error())
		return
	}

	if !allowDestination(host, port) {
		_, _ = fmt.Fprintf(conn, "ERROR forbidden\r\n")
		return
	}

	destAddr := net.JoinHostPort(host, strconv.Itoa(port))
	dialConn, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		_, _ = fmt.Fprintf(conn, "ERROR dial failed: %v\r\n", err)
		log.Printf("target dial failed %s: %v", destAddr, err)
		return
	}

	if _, err := fmt.Fprintf(conn, "OK\r\n"); err != nil {
		dialConn.Close()
		return
	}

	log.Printf("tunnel open: %s -> %s", conn.RemoteAddr().String(), destAddr)

	go relay(dialConn, conn)
	relay(conn, dialConn)
}

func relay(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()

	_, _ = io.Copy(dst, src)
}

func parseTarget(target string) (string, int, error) {
	if !strings.Contains(target, ":") {
		return "", 0, fmt.Errorf("target must be host:port")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}

func allowDestination(host string, port int) bool {
	allowedHosts := strings.Split(getEnv("NODE_ALLOWED_HOSTS", "example.com"), ",")
	for _, h := range allowedHosts {
		if strings.TrimSpace(strings.ToLower(h)) == strings.TrimSpace(strings.ToLower(host)) {
			if port == 443 || port == 80 {
				return true
			}
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
