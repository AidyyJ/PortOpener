package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestTokenLifecycle(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer store.Close()
	if err := store.ApplyMigrations(migrationsDir(t)); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	active, err := store.HasActiveToken()
	if err != nil {
		t.Fatalf("has active failed: %v", err)
	}
	if active {
		t.Fatalf("expected no active tokens")
	}

	if err := store.InsertToken("seed-token"); err != nil {
		t.Fatalf("insert token failed: %v", err)
	}
	ok, err := store.ValidateToken("seed-token")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected token valid")
	}

	if err := store.RotateToken("new-token"); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	ok, err = store.ValidateToken("seed-token")
	if err != nil {
		t.Fatalf("validate old failed: %v", err)
	}
	if ok {
		t.Fatalf("expected old token revoked")
	}
	ok, err = store.ValidateToken("new-token")
	if err != nil {
		t.Fatalf("validate new failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected new token valid")
	}
}

func TestUpsertHTTPReservationReplacesAllowlist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer store.Close()
	if err := store.ApplyMigrations(migrationsDir(t)); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	if err := store.UpsertTunnel(Tunnel{
		ID:        "tunnel-1",
		Protocol:  "http",
		LocalHost: "127.0.0.1",
		LocalPort: 8080,
		Status:    "active",
	}); err != nil {
		t.Fatalf("insert tunnel failed: %v", err)
	}

	res := HTTPReservation{TunnelID: "tunnel-1", Subdomain: "app", Allowlist: []string{"10.0.0.0/24"}}
	if err := store.UpsertHTTPReservation(res); err != nil {
		t.Fatalf("upsert reservation failed: %v", err)
	}
	res.Allowlist = []string{"192.168.1.0/24"}
	if err := store.UpsertHTTPReservation(res); err != nil {
		t.Fatalf("upsert reservation update failed: %v", err)
	}

	rows, err := store.db.Query("SELECT cidr FROM ip_allowlists WHERE tunnel_id = ? ORDER BY cidr ASC", "tunnel-1")
	if err != nil {
		t.Fatalf("query allowlists failed: %v", err)
	}
	defer rows.Close()

	var cidrs []string
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			t.Fatalf("scan allowlist failed: %v", err)
		}
		cidrs = append(cidrs, cidr)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("allowlist rows error: %v", err)
	}
	if len(cidrs) != 1 || cidrs[0] != "192.168.1.0/24" {
		t.Fatalf("expected allowlist replaced, got %v", cidrs)
	}

	var count int
	if err := store.db.QueryRow("SELECT COUNT(1) FROM ip_allowlists WHERE tunnel_id = ? AND cidr = ?", "tunnel-1", "10.0.0.0/24").Scan(&count); err != nil && err != sql.ErrNoRows {
		t.Fatalf("count allowlist failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected old allowlist removed, still present")
	}
}

func TestEnsureTokenRequiresSeed(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer store.Close()
	if err := store.ApplyMigrations(migrationsDir(t)); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	if err := store.EnsureToken(""); err != ErrTokenMissing {
		t.Fatalf("expected ErrTokenMissing, got %v", err)
	}
	if err := store.EnsureToken("seeded"); err != nil {
		t.Fatalf("ensure token failed: %v", err)
	}
	ok, err := store.ValidateToken("seeded")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected seeded token valid")
	}
}

func TestEnsureTokenIgnoresWhenActive(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer store.Close()
	if err := store.ApplyMigrations(migrationsDir(t)); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	if err := store.InsertToken("seed"); err != nil {
		t.Fatalf("insert token failed: %v", err)
	}
	if err := store.EnsureToken(""); err != nil {
		t.Fatalf("ensure token should no-op when active: %v", err)
	}
	if err := store.EnsureToken("another"); err != nil {
		t.Fatalf("ensure token should no-op when active: %v", err)
	}
	count, err := countActiveTokens(store)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active token, got %d", count)
	}
}

func countActiveTokens(store *Store) (int, error) {
	if store == nil || store.db == nil {
		return 0, os.ErrInvalid
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(1) FROM tokens WHERE revoked_at IS NULL").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func migrationsDir(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "migrations", "0001_initial.sql")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Join(dir, "migrations")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("migrations directory not found")
	return ""
}
