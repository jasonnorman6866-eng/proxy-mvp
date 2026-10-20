package policy

import "strings"

type Policy struct {
	allowedHosts map[string]bool
	allowedPorts map[int]bool
}

func New(hosts []string, ports []int) *Policy {
	p := &Policy{
		allowedHosts: make(map[string]bool),
		allowedPorts: make(map[int]bool),
	}
	for _, host := range hosts {
		p.allowedHosts[normalize(host)] = true
	}
	for _, port := range ports {
		p.allowedPorts[port] = true
	}
	return p
}

func (p *Policy) Allow(host string, port int) bool {
	if len(p.allowedPorts) > 0 && !p.allowedPorts[port] {
		return false
	}
	if len(p.allowedHosts) > 0 && !p.allowedHosts[normalize(host)] {
		return false
	}
	return true
}

func normalize(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimSuffix(host, ".")
	return host
}
