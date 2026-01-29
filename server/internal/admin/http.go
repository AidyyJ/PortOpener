package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/AidyyJ/PortOpener/server/internal/storage"
	"github.com/AidyyJ/PortOpener/server/internal/tunnels"
)

type API struct {
	Store *storage.Store
	Reg   *tunnels.Registry
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/reservations/http", a.withAuth(a.handleListHTTPReservations))
	mux.HandleFunc("/api/tunnels", a.withAuth(a.handleListTunnels))
	mux.HandleFunc("/api/tunnels/", a.withAuth(a.handleTunnelAction))
	mux.HandleFunc("/api/reservations/ports", a.withAuth(a.handleListPortReservations))
	mux.HandleFunc("/api/domains", a.withAuth(a.handleDomains))
	mux.HandleFunc("/api/tls/ask", a.handleTLSAsk)
	mux.HandleFunc("/api/logs", a.withAuth(a.handleListLogs))
	mux.HandleFunc("/api/metrics", a.withAuth(a.handleListMetrics))
	mux.HandleFunc("/api/token/rotate", a.withAuth(a.handleRotateToken))
	return mux
}

func (a *API) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Store == nil {
			http.Error(w, "store not configured", http.StatusServiceUnavailable)
			return
		}
		token := strings.TrimSpace(r.Header.Get("Authorization"))
		token = strings.TrimPrefix(token, "Bearer ")
		if token == "" {
			token = strings.TrimSpace(r.Header.Get("X-Admin-Token"))
		}
		ok, err := a.Store.ValidateToken(token)
		if err != nil {
			http.Error(w, "auth failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (a *API) handleListHTTPReservations(w http.ResponseWriter, _ *http.Request) {
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}

	reservations, err := a.Store.ListHTTPReservations()
	if err != nil {
		http.Error(w, "failed to list reservations", http.StatusInternalServerError)
		return
	}

	writeJSON(w, reservations)
}

func (a *API) handleListTunnels(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := parseLimit(r, 200)
	tunnels, err := a.Store.ListTunnels(limit)
	if err != nil {
		http.Error(w, "failed to list tunnels", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tunnels)
}

func (a *API) handleTunnelAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.NotFound(w, r)
		return
	}
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/tunnels/")
	if path == "" {
		http.Error(w, "tunnel id required", http.StatusBadRequest)
		return
	}
	if a.Reg != nil {
		_ = a.Reg.RemoveHTTPByTunnelID(path)
		_ = a.Reg.RemoveTCPByTunnelID(path)
		_ = a.Reg.RemoveUDPByTunnelID(path)
	}
	if err := a.Store.MarkTunnelStatus(path, "terminated"); err != nil {
		http.Error(w, "failed to update tunnel", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "terminated"})
}

func (a *API) handleListPortReservations(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := parseLimit(r, 200)
	protocol := strings.TrimSpace(r.URL.Query().Get("protocol"))
	ports, err := a.Store.ListPortReservations(protocol, limit)
	if err != nil {
		http.Error(w, "failed to list port reservations", http.StatusInternalServerError)
		return
	}
	writeJSON(w, ports)
}

func (a *API) handleDomains(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := parseLimit(r, 200)
		domains, err := a.Store.ListCustomDomains(limit)
		if err != nil {
			http.Error(w, "failed to list domains", http.StatusInternalServerError)
			return
		}
		writeJSON(w, domains)
		return
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		var payload storage.CustomDomain
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := a.Store.UpsertCustomDomain(payload); err != nil {
			http.Error(w, "failed to upsert domain", http.StatusInternalServerError)
			return
		}
		writeJSON(w, payload)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (a *API) handleTLSAsk(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if domain == "" {
		http.Error(w, "domain required", http.StatusBadRequest)
		return
	}
	entry, ok, err := a.Store.GetCustomDomain(domain)
	if err != nil {
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if !ok || strings.ToLower(entry.Status) != "enabled" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if entry.TunnelID == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	newToken, err := storage.GenerateToken()
	if err != nil {
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	if err := a.Store.RotateToken(newToken); err != nil {
		http.Error(w, "token rotation failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"token": newToken})
}

func (a *API) handleListLogs(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := parseLimit(r, 200)
	logs, err := a.Store.ListLogs(limit)
	if err != nil {
		http.Error(w, "failed to list logs", http.StatusInternalServerError)
		return
	}
	writeJSON(w, logs)
}

func (a *API) handleListMetrics(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := parseLimit(r, 200)
	metrics, err := a.Store.ListMetrics(limit)
	if err != nil {
		http.Error(w, "failed to list metrics", http.StatusInternalServerError)
		return
	}
	writeJSON(w, metrics)
}

func parseLimit(r *http.Request, fallback int) int {
	limit := fallback
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
