package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr      string
	BasicUser       string
	BasicPassword   string
	ExitNodeHost    string
	ExitNodePort    int
	AllowedHosts    []string
	AllowedPorts    []int
	RateLimit       int
	RateBurst       int
	KillSwitch      bool
	AuthorizationOK bool
}

func Load() Config {
	return Config{
		ListenAddr:      getEnv("GATEWAY_LISTEN_ADDR", ":8080"),
		BasicUser:       getEnv("GATEWAY_USERNAME", "demo"),
		BasicPassword:   getEnv("GATEWAY_PASSWORD", "demo"),
		ExitNodeHost:    getEnv("GATEWAY_EXIT_NODE_HOST", "127.0.0.1"),
		ExitNodePort:    getEnvInt("GATEWAY_EXIT_NODE_PORT", 9001),
		AllowedHosts:    csvToSlice(getEnv("GATEWAY_ALLOWED_HOSTS", "example.com")),
		AllowedPorts:    intsFromCSV(getEnv("GATEWAY_ALLOWED_PORTS", "443")),
		RateLimit:       getEnvInt("GATEWAY_RATE_LIMIT", 60),
		RateBurst:       getEnvInt("GATEWAY_RATE_BURST", 30),
		KillSwitch:      getEnvBool("GATEWAY_KILL_SWITCH", false),
		AuthorizationOK: true,
	}
}

func (c Config) IsAllowedDestination(host string, port int) bool {
	host = normalizeHost(host)
	if len(c.AllowedHosts) == 0 {
		return true
	}
	for _, allowed := range c.AllowedHosts {
		if normalizeHost(allowed) == host {
			return containsPort(c.AllowedPorts, port)
		}
	}
	return false
}

func normalizeHost(host string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.ToLower(host), "."))
}

func containsPort(ports []int, port int) bool {
	if len(ports) == 0 {
		return true
	}
	for _, candidate := range ports {
		if candidate == port {
			return true
		}
	}
	return false
}

func csvToSlice(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func intsFromCSV(value string) []int {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		i, err := strconv.Atoi(p)
		if err == nil {
			out = append(out, i)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return fallback
}
