package tunnels

import (
	"net"
	"strings"
)

type Allowlist struct {
	nets []*net.IPNet
}

func ParseAllowlist(values []string) (*Allowlist, error) {
	allow := &Allowlist{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		_, network, err := net.ParseCIDR(trimmed)
		if err != nil {
			return nil, err
		}
		allow.nets = append(allow.nets, network)
	}
	return allow, nil
}

func (a *Allowlist) Allows(remoteAddr string) bool {
	if a == nil || len(a.nets) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range a.nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
