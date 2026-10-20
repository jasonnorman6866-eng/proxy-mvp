package policy

import "strings"

type Policy struct {
	allowedPorts map[int]bool
}

func New(allowedPorts []int) *Policy {
	p := &Policy{allowedPorts: make(map[int]bool)}
	for _, port := range allowedPorts {
		p.allowedPorts[port] = true
	}
	return p
}

func (p *Policy) Allow(port int) bool {
	if len(p.allowedPorts) == 0 {
		return true
	}
	return p.allowedPorts[port]
}

func NormalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimSuffix(host, ".")
	return host
}
