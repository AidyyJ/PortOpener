package relayserver

import (
	"net"
	"testing"
	"time"
)

func TestUDPCleanupSessionsRemovesExpired(t *testing.T) {
	oldClient, oldServer := net.Pipe()
	newClient, newServer := net.Pipe()
	defer oldClient.Close()
	defer oldServer.Close()
	defer newClient.Close()
	defer newServer.Close()

	port := 20000
	proxy := &UDPProxy{
		sessions: map[int]map[string]*udpSession{
			port: {
				"old": {stream: oldServer, lastSeen: time.Now().Add(-udpIdleTimeout - time.Second)},
				"new": {stream: newServer, lastSeen: time.Now()},
			},
		},
		lastClean: time.Now().Add(-udpCleanupEvery - time.Second),
	}

	proxy.cleanupSessions(port)
	if _, ok := proxy.sessions[port]["old"]; ok {
		t.Fatalf("expected old session removed")
	}
	if _, ok := proxy.sessions[port]["new"]; !ok {
		t.Fatalf("expected new session retained")
	}
}

func TestUDPCleanupSessionsSkipsWhenRecent(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	port := 20001
	proxy := &UDPProxy{
		sessions: map[int]map[string]*udpSession{
			port: {
				"old": {stream: server, lastSeen: time.Now().Add(-udpIdleTimeout - time.Second)},
			},
		},
		lastClean: time.Now(),
	}

	proxy.cleanupSessions(port)
	if _, ok := proxy.sessions[port]["old"]; !ok {
		t.Fatalf("expected session retained when cleanup interval not reached")
	}
}
