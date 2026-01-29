package tunnels

import (
	"errors"
	"strings"
	"sync"

	"github.com/hashicorp/yamux"
)

var ErrTunnelExists = errors.New("tunnel already registered")

type HTTPRegistration struct {
	Subdomain string
	Allowlist []string
}

type HTTPEntry struct {
	TunnelID  string
	Subdomain string
	Allowlist []string
	Session   *yamux.Session
}

type TCPEntry struct {
	TunnelID     string
	ExternalPort int
	Session      *yamux.Session
}

type UDPEntry struct {
	TunnelID     string
	ExternalPort int
	Session      *yamux.Session
}

type Registry struct {
	mu      sync.RWMutex
	httpMap map[string]HTTPEntry
	tcpMap  map[int]TCPEntry
	udpMap  map[int]UDPEntry
}

func NewRegistry() *Registry {
	return &Registry{httpMap: make(map[string]HTTPEntry), tcpMap: make(map[int]TCPEntry), udpMap: make(map[int]UDPEntry)}
}

func (r *Registry) RegisterHTTP(tunnelID string, session *yamux.Session, reg HTTPRegistration) error {
	key := strings.ToLower(strings.TrimSpace(reg.Subdomain))
	if key == "" {
		return errors.New("subdomain required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.httpMap[key]; exists {
		return ErrTunnelExists
	}

	r.httpMap[key] = HTTPEntry{
		TunnelID:  tunnelID,
		Subdomain: key,
		Allowlist: reg.Allowlist,
		Session:   session,
	}
	return nil
}

func (r *Registry) RemoveHTTP(subdomain string) {
	key := strings.ToLower(strings.TrimSpace(subdomain))
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.httpMap, key)
}

func (r *Registry) RemoveHTTPByTunnelID(tunnelID string) []HTTPEntry {
	if tunnelID == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var removed []HTTPEntry
	for key, entry := range r.httpMap {
		if entry.TunnelID == tunnelID {
			removed = append(removed, entry)
			delete(r.httpMap, key)
		}
	}
	return removed
}

func (r *Registry) LookupHTTP(subdomain string) (HTTPEntry, bool) {
	key := strings.ToLower(strings.TrimSpace(subdomain))
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.httpMap[key]
	return entry, ok
}

func (r *Registry) LookupHTTPByTunnelID(tunnelID string) (HTTPEntry, bool) {
	if tunnelID == "" {
		return HTTPEntry{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.httpMap {
		if entry.TunnelID == tunnelID {
			return entry, true
		}
	}
	return HTTPEntry{}, false
}

func (r *Registry) ListHTTP() []HTTPEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]HTTPEntry, 0, len(r.httpMap))
	for _, entry := range r.httpMap {
		entries = append(entries, entry)
	}
	return entries
}

func (r *Registry) RegisterTCP(tunnelID string, session *yamux.Session, externalPort int) error {
	if tunnelID == "" {
		return errors.New("tunnel id required")
	}
	if externalPort == 0 {
		return errors.New("external port required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, exists := r.tcpMap[externalPort]; exists && existing.TunnelID != tunnelID {
		return ErrTunnelExists
	}
	r.tcpMap[externalPort] = TCPEntry{TunnelID: tunnelID, ExternalPort: externalPort, Session: session}
	return nil
}

func (r *Registry) LookupTCP(externalPort int) (TCPEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.tcpMap[externalPort]
	return entry, ok
}

func (r *Registry) RemoveTCP(externalPort int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tcpMap, externalPort)
}

func (r *Registry) RemoveTCPByTunnelID(tunnelID string) []TCPEntry {
	if tunnelID == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var removed []TCPEntry
	for port, entry := range r.tcpMap {
		if entry.TunnelID == tunnelID {
			removed = append(removed, entry)
			delete(r.tcpMap, port)
		}
	}
	return removed
}

func (r *Registry) ListTCP() []TCPEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]TCPEntry, 0, len(r.tcpMap))
	for _, entry := range r.tcpMap {
		entries = append(entries, entry)
	}
	return entries
}

func (r *Registry) RegisterUDP(tunnelID string, session *yamux.Session, externalPort int) error {
	if tunnelID == "" {
		return errors.New("tunnel id required")
	}
	if externalPort == 0 {
		return errors.New("external port required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, exists := r.udpMap[externalPort]; exists && existing.TunnelID != tunnelID {
		return ErrTunnelExists
	}
	r.udpMap[externalPort] = UDPEntry{TunnelID: tunnelID, ExternalPort: externalPort, Session: session}
	return nil
}

func (r *Registry) LookupUDP(externalPort int) (UDPEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.udpMap[externalPort]
	return entry, ok
}

func (r *Registry) RemoveUDP(externalPort int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.udpMap, externalPort)
}

func (r *Registry) RemoveUDPByTunnelID(tunnelID string) []UDPEntry {
	if tunnelID == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var removed []UDPEntry
	for port, entry := range r.udpMap {
		if entry.TunnelID == tunnelID {
			removed = append(removed, entry)
			delete(r.udpMap, port)
		}
	}
	return removed
}

func (r *Registry) ListUDP() []UDPEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]UDPEntry, 0, len(r.udpMap))
	for _, entry := range r.udpMap {
		entries = append(entries, entry)
	}
	return entries
}
