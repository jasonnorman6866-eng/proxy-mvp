package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"proxy-mvp/internal/audit"
	"proxy-mvp/internal/auth"
	"proxy-mvp/internal/meter"
	"proxy-mvp/internal/policy"
	"proxy-mvp/internal/proxy"
)

type Config struct {
	Gateway struct {
		Listen string `yaml:"listen"`
	} `yaml:"gateway"`
	Auth struct {
		Enabled  bool   `yaml:"enabled"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"auth"`
	Policy struct {
		AllowedPorts []int `yaml:"allowed_ports"`
	} `yaml:"policy"`
	Egress struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"egress"`
	Limits struct {
		MaxConnectionsPerUser int `yaml:"max_connections_per_user"`
	} `yaml:"limits"`
	Audit struct {
		Enabled     bool `yaml:"enabled"`
		LogPayloads bool `yaml:"log_payloads"`
	} `yaml:"audit"`
}

func loadConfig(path string) (Config, error) {
	cfg := Config{}
	if path == "" {
		path = "config/config.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Gateway.Listen == "" {
		cfg.Gateway.Listen = ":8080"
	}
	if cfg.Auth.Username == "" {
		cfg.Auth.Username = "demo"
	}
	if cfg.Auth.Password == "" {
		cfg.Auth.Password = "demo"
	}
	if len(cfg.Policy.AllowedPorts) == 0 {
		cfg.Policy.AllowedPorts = []int{443}
	}
	if cfg.Limits.MaxConnectionsPerUser == 0 {
		cfg.Limits.MaxConnectionsPerUser = 5
	}
	return cfg, nil
}

func main() {
	cfg, err := loadConfig(os.Getenv("GATEWAY_CONFIG"))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pol := policy.New(cfg.Policy.AllowedPorts)
	met := meter.New()
	logger := audit.New(cfg.Audit.Enabled, cfg.Audit.LogPayloads)
	srv := &proxy.Server{
		AuthEnabled:             cfg.Auth.Enabled,
		Username:                cfg.Auth.Username,
		Password:                cfg.Auth.Password,
		Policy:                  pol,
		Meter:                   met,
		Logger:                  logger,
		EgressEnabled:           cfg.Egress.Enabled,
		MaxConnectionsPerUser:   cfg.Limits.MaxConnectionsPerUser,
		MaxConnectionsPerHandle: 5,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"ok"}`)
	})
	mux.Handle("/", srv)

	log.Printf("gateway listening on %s", cfg.Gateway.Listen)
	server := &http.Server{
		Addr:              cfg.Gateway.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       0,
	}
	if err := server.ListenAndServe(); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		log.Fatalf("serve gateway: %v", err)
	}
}
