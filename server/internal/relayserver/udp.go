package relayserver

import (
	"encoding/base64"
	"net"
	"sync"
	"time"

	"github.com/AidyyJ/PortOpener/internal/relay"
	"github.com/AidyyJ/PortOpener/server/internal/storage"
	"github.com/AidyyJ/PortOpener/server/internal/tunnels"
)

type UDPProxy struct {
	Registry *tunnels.Registry
	Store    *storage.Store

	mu        sync.Mutex
	conns     map[int]*net.UDPConn
	sessions  map[int]map[string]*udpSession
	lastClean time.Time
}

type udpSession struct {
	stream   net.Conn
	remote   *net.UDPAddr
	lastSeen time.Time
	mu       sync.Mutex
}

const (
	udpIdleTimeout  = 2 * time.Minute
	udpCleanupEvery = 30 * time.Second
)

func (p *UDPProxy) EnsureListener(port int) error {
	if port <= 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conns == nil {
		p.conns = make(map[int]*net.UDPConn)
	}
	if p.sessions == nil {
		p.sessions = make(map[int]map[string]*udpSession)
	}
	if _, exists := p.conns[port]; exists {
		return nil
	}
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	p.conns[port] = conn
	p.sessions[port] = make(map[string]*udpSession)
	go p.readLoop(port, conn)
	return nil
}

func (p *UDPProxy) RemoveListener(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	conn, ok := p.conns[port]
	if !ok {
		return
	}
	if sessions, ok := p.sessions[port]; ok {
		for _, session := range sessions {
			_ = session.stream.Close()
		}
	}
	_ = conn.Close()
	delete(p.conns, port)
	delete(p.sessions, port)
}

func (p *UDPProxy) readLoop(port int, conn *net.UDPConn) {
	buf := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		payload := make([]byte, n)
		copy(payload, buf[:n])
		p.handleDatagram(port, conn, addr, payload)
	}
}

func (p *UDPProxy) handleDatagram(port int, conn *net.UDPConn, addr *net.UDPAddr, payload []byte) {
	if p.Registry == nil {
		return
	}
	entry, ok := p.Registry.LookupUDP(port)
	if !ok || entry.Session == nil {
		return
	}
	remote := addr.String()
	session := p.getOrCreateSession(port, remote, entry, addr)
	if session == nil {
		return
	}
	session.lastSeen = time.Now().UTC()
	session.mu.Lock()
	err := relay.WriteJSON(session.stream, relay.UDPDatagram{RemoteAddr: remote, PayloadB64: base64.StdEncoding.EncodeToString(payload)})
	session.mu.Unlock()
	if err != nil {
		p.dropSession(port, remote)
		return
	}
	if p.Store != nil {
		_ = p.Store.InsertLog(storage.LogEntry{
			TunnelID:   entry.TunnelID,
			Timestamp:  time.Now().UTC(),
			Kind:       "udp",
			RemoteAddr: remote,
			Summary:    "udp port " + itoa(port),
			BytesIn:    int64(len(payload)),
		})
		_ = p.Store.AddMetric(entry.TunnelID, time.Now().UTC(), 0, 1, int64(len(payload)), 0)
	}
	p.cleanupSessions(port)
}

func (p *UDPProxy) getOrCreateSession(port int, remote string, entry tunnels.UDPEntry, addr *net.UDPAddr) *udpSession {
	p.mu.Lock()
	sessions := p.sessions[port]
	if session, ok := sessions[remote]; ok {
		session.lastSeen = time.Now().UTC()
		p.mu.Unlock()
		return session
	}
	p.mu.Unlock()

	stream, err := entry.Session.OpenStream()
	if err != nil {
		return nil
	}
	session := &udpSession{stream: stream, remote: addr, lastSeen: time.Now().UTC()}
	p.mu.Lock()
	if existing, ok := p.sessions[port][remote]; ok {
		p.mu.Unlock()
		_ = stream.Close()
		return existing
	}
	p.sessions[port][remote] = session
	p.mu.Unlock()
	go p.readResponses(port, remote, session)
	return session
}

func (p *UDPProxy) readResponses(port int, remote string, session *udpSession) {
	defer func() {
		_ = session.stream.Close()
		p.dropSession(port, remote)
	}()
	var tunnelID string
	if p.Registry != nil {
		if entry, ok := p.Registry.LookupUDP(port); ok {
			tunnelID = entry.TunnelID
		}
	}
	for {
		var resp relay.UDPDatagram
		if err := relay.ReadJSON(session.stream, &resp); err != nil {
			return
		}
		data, err := base64.StdEncoding.DecodeString(resp.PayloadB64)
		if err != nil {
			return
		}
		p.mu.Lock()
		conn := p.conns[port]
		p.mu.Unlock()
		if conn == nil {
			return
		}
		_, _ = conn.WriteToUDP(data, session.remote)
		session.lastSeen = time.Now().UTC()
		if p.Store != nil {
			_ = p.Store.AddMetric(tunnelID, time.Now().UTC(), 0, 1, 0, int64(len(data)))
		}
	}
}

func (p *UDPProxy) dropSession(port int, remote string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sessions, ok := p.sessions[port]; ok {
		delete(sessions, remote)
	}
}

func (p *UDPProxy) cleanupSessions(port int) {
	if time.Since(p.lastClean) < udpCleanupEvery {
		return
	}
	p.lastClean = time.Now().UTC()
	p.mu.Lock()
	sessions := p.sessions[port]
	for remote, session := range sessions {
		if time.Since(session.lastSeen) > udpIdleTimeout {
			_ = session.stream.Close()
			delete(sessions, remote)
		}
	}
	p.mu.Unlock()
}
