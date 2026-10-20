package main

import (
	"fmt"
	"log"
	"net/http"

	"proxy-mvp/internal/config"
	"proxy-mvp/internal/proxy"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	handler := proxy.New(cfg)
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Gateway.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
	mux.Handle("/", handler)

	server := &http.Server{
		Addr:    cfg.Gateway.Listen,
		Handler: mux,
	}

	log.Printf("gateway listening on %s", cfg.Gateway.Listen)
	log.Printf("exit node: %s:%d authorized=%t healthy=%t kill_switch=%t", cfg.ExitNode.Host, cfg.ExitNode.Port, cfg.ExitNode.Authorized, cfg.ExitNode.Healthy, cfg.ExitNode.KillSwitch)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err)
	}
}
