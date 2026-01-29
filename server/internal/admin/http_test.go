package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowAdminIP(t *testing.T) {
	api := &API{AdminAllowlist: "10.0.0.0/24"}

	req := httptest.NewRequest(http.MethodGet, "/api/tunnels", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	if !api.allowAdminIP(req) {
		t.Fatalf("expected allowlist to allow remote addr")
	}

	blocked := httptest.NewRequest(http.MethodGet, "/api/tunnels", nil)
	blocked.RemoteAddr = "192.168.1.5:5555"
	if api.allowAdminIP(blocked) {
		t.Fatalf("expected allowlist to block remote addr")
	}
}

func TestAllowAdminIPUsesForwardedFor(t *testing.T) {
	api := &API{AdminAllowlist: "203.0.113.0/24"}
	req := httptest.NewRequest(http.MethodGet, "/api/tunnels", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 70.41.3.18")
	if !api.allowAdminIP(req) {
		t.Fatalf("expected allowlist to use X-Forwarded-For first hop")
	}
}
