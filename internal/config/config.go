package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Gateway  Gateway  `yaml:"gateway"`
	ExitNode ExitNode `yaml:"exit_node"`
	Policy   Policy   `yaml:"policy"`
	Meter    Meter    `yaml:"meter"`
	Audit    Audit    `yaml:"audit"`
}

type Gateway struct {
	Listen     string `yaml:"listen"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	HealthPath string `yaml:"health_path"`
}

type ExitNode struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Label      string `yaml:"label"`
	Authorized bool   `yaml:"authorized"`
	Healthy    bool   `yaml:"healthy"`
	KillSwitch bool   `yaml:"kill_switch"`
}

type Policy struct {
	AllowedHosts []string `yaml:"allowed_hosts"`
	AllowedPorts []int    `yaml:"allowed_ports"`
}

type Meter struct {
	RateLimitPerMinute int `yaml:"rate_limit_per_minute"`
	Burst              int `yaml:"burst"`
}

type Audit struct {
	Enabled bool `yaml:"enabled"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "config/config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	if cfg.Gateway.Listen == "" {
		cfg.Gateway.Listen = ":8080"
	}
	if cfg.Gateway.Username == "" {
		cfg.Gateway.Username = "demo"
	}
	if cfg.Gateway.Password == "" {
		cfg.Gateway.Password = "demo"
	}
	if cfg.Gateway.HealthPath == "" {
		cfg.Gateway.HealthPath = "/health"
	}
	if cfg.ExitNode.Host == "" {
		cfg.ExitNode.Host = "example.com"
	}
	if cfg.ExitNode.Port == 0 {
		cfg.ExitNode.Port = 443
	}
	if cfg.ExitNode.Label == "" {
		cfg.ExitNode.Label = "authorized-test-egress"
	}
	if !cfg.ExitNode.Authorized && !cfg.ExitNode.Healthy {
		cfg.ExitNode.Authorized = true
		cfg.ExitNode.Healthy = true
	}
	if len(cfg.Policy.AllowedHosts) == 0 {
		cfg.Policy.AllowedHosts = []string{"example.com"}
	}
	if len(cfg.Policy.AllowedPorts) == 0 {
		cfg.Policy.AllowedPorts = []int{80, 443}
	}
	if cfg.Meter.RateLimitPerMinute <= 0 {
		cfg.Meter.RateLimitPerMinute = 60
	}
	if cfg.Meter.Burst <= 0 {
		cfg.Meter.Burst = 30
	}
	return cfg, nil
}
