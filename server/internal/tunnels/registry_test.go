package tunnels

import "testing"

func TestRegistryRegisterHTTPDuplicate(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterHTTP("t1", nil, HTTPRegistration{Subdomain: "app"}); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := registry.RegisterHTTP("t2", nil, HTTPRegistration{Subdomain: "app"}); err != ErrTunnelExists {
		t.Fatalf("expected ErrTunnelExists, got %v", err)
	}
}

func TestRegistryRegisterTCPConflict(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterTCP("t1", nil, 25000); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := registry.RegisterTCP("t2", nil, 25000); err != ErrTunnelExists {
		t.Fatalf("expected ErrTunnelExists, got %v", err)
	}
}

func TestRegistryRemoveHTTPByTunnelID(t *testing.T) {
	registry := NewRegistry()
	_ = registry.RegisterHTTP("t1", nil, HTTPRegistration{Subdomain: "app"})
	_ = registry.RegisterHTTP("t1", nil, HTTPRegistration{Subdomain: "api"})
	_ = registry.RegisterHTTP("t2", nil, HTTPRegistration{Subdomain: "other"})

	removed := registry.RemoveHTTPByTunnelID("t1")
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed entries, got %d", len(removed))
	}
	if _, ok := registry.LookupHTTP("app"); ok {
		t.Fatalf("expected app removed")
	}
	if _, ok := registry.LookupHTTP("other"); !ok {
		t.Fatalf("expected other to remain")
	}
}
