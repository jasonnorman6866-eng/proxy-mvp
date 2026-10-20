package main

import (
	"log"
	"net/http"
	"time"

	"proxy-mvp/internal/config"
	"proxy-mvp/internal/proxy"
)

func main() {
	cfg := config.Load()
	p := proxy.New(cfg)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           p,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      0,
		ReadTimeout:       0,
		IdleTimeout:       0,
	}

	log.Printf("gateway listening on %s", cfg.ListenAddr)
	log.Printf("auth user=%s exit node=%s:%d allowed hosts=%v allowed ports=%v", cfg.BasicUser, cfg.ExitNodeHost, cfg.ExitNodePort, cfg.AllowedHosts, cfg.AllowedPorts)
	log.Fatal(server.ListenAndServe())
}
