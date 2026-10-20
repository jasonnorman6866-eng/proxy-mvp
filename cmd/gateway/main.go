package main

import (
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
	log.Printf("gateway listening on %s", cfg.Gateway.Listen)
	log.Printf("exit node: %s:%d authorized=%t healthy=%t kill_switch=%t", cfg.ExitNode.Host, cfg.ExitNode.Port, cfg.ExitNode.Authorized, cfg.ExitNode.Healthy, cfg.ExitNode.KillSwitch)

	if err := http.ListenAndServe(cfg.Gateway.Listen, handler); err != nil {
		log.Fatalf("serve gateway: %v", err)
	}
}
