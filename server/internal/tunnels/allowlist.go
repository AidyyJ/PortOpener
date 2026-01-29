package tunnels

import (
	"net"
	"strings"
)

type Allowlist struct {
	nets []*net.IPNet
}

type ParsedAllowlist struct {
	Allowlist *Allowlist
	Any       bool
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

func ParseAllowlistCSV(values string) (*ParsedAllowlist, error) {
	trimmed := strings.TrimSpace(values)
	if trimmed == "" {
		return &ParsedAllowlist{Any: true}, nil
	}
	allow, err := ParseAllowlist(strings.Split(values, ","))
	if err != nil {
		return nil, err
	}
	if allow == nil || len(allow.nets) == 0 {
		return &ParsedAllowlist{Any: true}, nil
	}
	return &ParsedAllowlist{Allowlist: allow}, nil
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
