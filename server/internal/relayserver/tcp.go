package relayserver

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/AidyyJ/PortOpener/internal/relay"
	"github.com/AidyyJ/PortOpener/server/internal/storage"
	"github.com/AidyyJ/PortOpener/server/internal/tunnels"
)

type TCPProxy struct {
	Registry  *tunnels.Registry
	Store     *storage.Store
	listeners map[int]net.Listener
	mu        sync.Mutex
}

func (p *TCPProxy) EnsureListener(port int) error {
	if port <= 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.listeners == nil {
		p.listeners = make(map[int]net.Listener)
	}
	if _, exists := p.listeners[port]; exists {
		return nil
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("", itoa(port)))
	if err != nil {
		return err
	}
	p.listeners[port] = ln
	go p.acceptLoop(port, ln)
	return nil
}

func (p *TCPProxy) RemoveListener(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ln, ok := p.listeners[port]
	if !ok {
		return
	}
	_ = ln.Close()
	delete(p.listeners, port)
}

func (p *TCPProxy) acceptLoop(port int, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go p.handleConn(port, conn)
	}
}

func (p *TCPProxy) handleConn(port int, conn net.Conn) {
	defer conn.Close()
	if p.Registry == nil {
		return
	}
	entry, ok := p.Registry.LookupTCP(port)
	if !ok || entry.Session == nil {
		return
	}
	stream, err := entry.Session.OpenStream()
	if err != nil {
		return
	}
	defer stream.Close()

	_ = relay.WriteJSON(stream, relay.ControlMessage{Type: "tcp_open", TunnelID: entry.TunnelID, ExternalPort: port})

	var bytesIn int64
	var bytesOut int64
	copyErr := make(chan error, 2)
	go func() {
		count, err := io.Copy(stream, conn)
		bytesIn = count
		copyErr <- err
	}()
	go func() {
		count, err := io.Copy(conn, stream)
		bytesOut = count
		copyErr <- err
	}()
	<-copyErr

	if p.Store != nil {
		_ = p.Store.InsertLog(storage.LogEntry{
			TunnelID:   entry.TunnelID,
			Timestamp:  time.Now().UTC(),
			Kind:       "tcp",
			RemoteAddr: conn.RemoteAddr().String(),
			Summary:    "tcp port " + itoa(port),
			BytesIn:    bytesIn,
			BytesOut:   bytesOut,
		})
		_ = p.Store.AddMetric(entry.TunnelID, time.Now().UTC(), 0, 1, bytesIn, bytesOut)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	neg := value < 0
	if neg {
		value = -value
	}
	var buf [12]byte
	idx := len(buf)
	for value > 0 {
		idx--
		buf[idx] = byte('0' + value%10)
		value /= 10
	}
	if neg {
		idx--
		buf[idx] = '-'
	}
	return string(buf[idx:])
}
