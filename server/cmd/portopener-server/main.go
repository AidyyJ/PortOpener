package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AidyyJ/PortOpener/server/internal/admin"
	"github.com/AidyyJ/PortOpener/server/internal/metrics"
	"github.com/AidyyJ/PortOpener/server/internal/relayserver"
	"github.com/AidyyJ/PortOpener/server/internal/storage"
	"github.com/AidyyJ/PortOpener/server/internal/tunnels"
)

func main() {
	addr := getenv("PORTOPENER_HTTP_ADDR", ":8080")
	webRoot := getenv("PORTOPENER_WEB_ROOT", "web")

	adminDir := filepath.Join(webRoot, "admin")
	adminFS := http.FileServer(http.Dir(adminDir))
	publicFS := http.FileServer(http.Dir(webRoot))
	dbPath := getenv("PORTOPENER_DB_PATH", "data/portopener.db")
	migrationsDir := getenv("PORTOPENER_MIGRATIONS_DIR", "migrations")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	relayToken := firstNonEmpty(
		getenv("PORTOPENER_RELAY_TOKEN", ""),
		getenv("PORTOPENER_ADMIN_TOKEN", ""),
	)
	registry := tunnels.NewRegistry()
	collector := metrics.New()
	logger := metrics.NewLogger(1000)
	store, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	defer store.Close()
	if err := store.ApplyMigrations(migrationsDir); err != nil {
		log.Fatalf("db migration failed: %v", err)
	}
	if err := store.EnsureToken(relayToken); err != nil {
		log.Fatalf("token init failed: %v", err)
	}
	relaySrv := relayserver.New(relayserver.Config{Token: relayToken}, registry, store)
	adminAPI := &admin.API{Store: store, Reg: registry}
	proxy := &relayserver.HTTPProxy{Registry: registry, Metrics: collector, Logs: logger, Store: store}

	mux.HandleFunc("/relay", relaySrv.Handler())
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/proxy/") {
			proxy.Handler().ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			adminAPI.Handler().ServeHTTP(w, r)
			return
		}
		if isAdminHost(r.Host) {
			adminFS.ServeHTTP(w, r)
			return
		}
		publicFS.ServeHTTP(w, r)
	}))

	log.Printf("portopener-server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("listen failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isAdminHost(hostport string) bool {
	host := hostport
	if strings.Contains(hostport, ":") {
		if parsedHost, _, err := net.SplitHostPort(hostport); err == nil {
			host = parsedHost
		}
	}
	host = strings.ToLower(host)
	return strings.HasPrefix(host, "admin.")
}
