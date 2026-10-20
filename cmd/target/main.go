package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	listenAddr := env("TARGET_LISTEN_ADDR", ":9444")
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen target: %v", err)
	}
	defer ln.Close()
	log.Printf("local target listening on %s", listenAddr)
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
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			_, _ = conn.Write([]byte(fmt.Sprintf("echo:%s", string(buf[:n]))))
		}
		if err != nil {
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
