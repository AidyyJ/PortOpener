package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

var ErrTokenMissing = errors.New("relay token not configured")

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("db path required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *Store) EnsureToken(raw string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not configured")
	}
	active, err := s.HasActiveToken()
	if err != nil {
		return err
	}
	if active {
		return nil
	}
	if strings.TrimSpace(raw) == "" {
		return ErrTokenMissing
	}
	return s.InsertToken(raw)
}

func (s *Store) HasActiveToken() (bool, error) {
	if s == nil || s.db == nil {
		return false, nil
	}
	var count int
	if err := s.db.QueryRow("SELECT COUNT(1) FROM tokens WHERE revoked_at IS NULL").Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) ValidateToken(raw string) (bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false, nil
	}
	hash := hashToken(trimmed)
	var count int
	if err := s.db.QueryRow("SELECT COUNT(1) FROM tokens WHERE token_hash = ? AND revoked_at IS NULL", hash).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) InsertToken(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("token required")
	}
	hash := hashToken(trimmed)
	_, err := s.db.Exec("INSERT INTO tokens (token_hash, created_at) VALUES (?, ?)", hash, nowUTC())
	return err
}

func (s *Store) RotateToken(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("token required")
	}
	hash := hashToken(trimmed)
	if s == nil || s.db == nil {
		return fmt.Errorf("store not configured")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE tokens SET revoked_at = ? WHERE revoked_at IS NULL", nowUTC()); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO tokens (token_hash, created_at) VALUES (?, ?)", hash, nowUTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func hashToken(raw string) string {
	bytes := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(bytes[:])
}

type HTTPReservation struct {
	TunnelID  string
	Subdomain string
	Allowlist []string
}

type Tunnel struct {
	ID        string
	Name      string
	Protocol  string
	LocalHost string
	LocalPort int
	Status    string
	CreatedAt time.Time
	LastSeen  time.Time
}

type PortReservation struct {
	Protocol     string
	ExternalPort int
	TunnelID     string
	Reserved     bool
	CreatedAt    time.Time
}

type CustomDomain struct {
	Domain    string
	TunnelID  string
	Status    string
	CertState string
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LogEntry struct {
	TunnelID   string
	Timestamp  time.Time
	Kind       string
	RemoteAddr string
	Summary    string
	Status     int
	BytesIn    int64
	BytesOut   int64
}

type MetricRollup struct {
	TunnelID     string
	MinuteBucket int64
	ReqCount     int64
	ConnCount    int64
	BytesIn      int64
	BytesOut     int64
}

func (s *Store) UpsertHTTPReservation(res HTTPReservation) error {
	if res.Subdomain == "" {
		return fmt.Errorf("subdomain required")
	}

	if _, err := s.db.Exec(`INSERT INTO subdomains (subdomain, tunnel_id, reserved, created_at)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(subdomain) DO UPDATE SET tunnel_id = excluded.tunnel_id`, res.Subdomain, res.TunnelID, nowUTC()); err != nil {
		return err
	}

	if res.TunnelID != "" {
		if _, err := s.db.Exec("DELETE FROM ip_allowlists WHERE tunnel_id = ?", res.TunnelID); err != nil {
			return err
		}
		for _, cidr := range res.Allowlist {
			if _, err := s.db.Exec("INSERT INTO ip_allowlists (tunnel_id, cidr, created_at) VALUES (?, ?, ?)", res.TunnelID, cidr, nowUTC()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) UpsertTunnel(tunnel Tunnel) error {
	if tunnel.ID == "" {
		return fmt.Errorf("tunnel id required")
	}
	if tunnel.Protocol == "" {
		return fmt.Errorf("protocol required")
	}
	if tunnel.Status == "" {
		tunnel.Status = "active"
	}
	createdAt := tunnel.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	lastSeen := tunnel.LastSeen
	if lastSeen.IsZero() {
		lastSeen = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO tunnels (id, name, protocol, local_host, local_port, status, created_at, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			protocol = excluded.protocol,
			local_host = excluded.local_host,
			local_port = excluded.local_port,
			status = excluded.status,
			last_seen = excluded.last_seen`,
		tunnel.ID,
		tunnel.Name,
		tunnel.Protocol,
		tunnel.LocalHost,
		tunnel.LocalPort,
		tunnel.Status,
		createdAt.UTC().Format(time.RFC3339),
		lastSeen.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) MarkTunnelStatus(tunnelID, status string) error {
	if tunnelID == "" {
		return fmt.Errorf("tunnel id required")
	}
	if status == "" {
		status = "inactive"
	}
	_, err := s.db.Exec(`UPDATE tunnels SET status = ?, last_seen = ? WHERE id = ?`, status, nowUTC(), tunnelID)
	return err
}

func (s *Store) ListTunnels(limit int) ([]Tunnel, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT id, name, protocol, local_host, local_port, status, created_at, last_seen
		FROM tunnels ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Tunnel
	for rows.Next() {
		var entry Tunnel
		var createdAt, lastSeen string
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.Protocol, &entry.LocalHost, &entry.LocalPort, &entry.Status, &createdAt, &lastSeen); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			entry.CreatedAt = parsed
		}
		if parsed, err := time.Parse(time.RFC3339, lastSeen); err == nil {
			entry.LastSeen = parsed
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (s *Store) UpsertPortReservation(res PortReservation) error {
	if res.Protocol == "" {
		return fmt.Errorf("protocol required")
	}
	if res.ExternalPort == 0 {
		return fmt.Errorf("external port required")
	}
	createdAt := res.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	reserved := 1
	if !res.Reserved {
		reserved = 0
	}
	_, err := s.db.Exec(`INSERT INTO port_reservations (protocol, external_port, tunnel_id, reserved, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(protocol, external_port) DO UPDATE SET
			tunnel_id = excluded.tunnel_id,
			reserved = excluded.reserved`, res.Protocol, res.ExternalPort, res.TunnelID, reserved, createdAt.UTC().Format(time.RFC3339))
	return err
}

func (s *Store) ListPortReservations(protocol string, limit int) ([]PortReservation, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT protocol, external_port, tunnel_id, reserved, created_at
		FROM port_reservations`
	args := []any{}
	if protocol != "" {
		query += " WHERE protocol = ?"
		args = append(args, protocol)
	}
	query += " ORDER BY external_port ASC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PortReservation
	for rows.Next() {
		var entry PortReservation
		var createdAt string
		var reservedInt int
		if err := rows.Scan(&entry.Protocol, &entry.ExternalPort, &entry.TunnelID, &reservedInt, &createdAt); err != nil {
			return nil, err
		}
		entry.Reserved = reservedInt != 0
		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			entry.CreatedAt = parsed
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (s *Store) GetPortReservation(protocol string, externalPort int) (PortReservation, bool, error) {
	var entry PortReservation
	if strings.TrimSpace(protocol) == "" || externalPort == 0 {
		return entry, false, nil
	}
	var createdAt string
	var reservedInt int
	err := s.db.QueryRow(`SELECT protocol, external_port, tunnel_id, reserved, created_at
		FROM port_reservations WHERE protocol = ? AND external_port = ?`, protocol, externalPort).
		Scan(&entry.Protocol, &entry.ExternalPort, &entry.TunnelID, &reservedInt, &createdAt)
	if err == sql.ErrNoRows {
		return entry, false, nil
	}
	if err != nil {
		return entry, false, err
	}
	entry.Reserved = reservedInt != 0
	if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
		entry.CreatedAt = parsed
	}
	return entry, true, nil
}

func (s *Store) UpsertCustomDomain(domain CustomDomain) error {
	cleanDomain := strings.ToLower(strings.TrimSpace(domain.Domain))
	if cleanDomain == "" {
		return fmt.Errorf("domain required")
	}
	domain.Domain = cleanDomain
	if domain.Status == "" {
		domain.Status = "pending"
	}
	createdAt := domain.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := domain.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO custom_domains (domain, tunnel_id, status, cert_state, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET
			tunnel_id = excluded.tunnel_id,
			status = excluded.status,
			cert_state = excluded.cert_state,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at`,
		domain.Domain,
		domain.TunnelID,
		domain.Status,
		domain.CertState,
		domain.LastError,
		createdAt.UTC().Format(time.RFC3339),
		updatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetCustomDomain(domain string) (CustomDomain, bool, error) {
	var entry CustomDomain
	cleanDomain := strings.ToLower(strings.TrimSpace(domain))
	if cleanDomain == "" {
		return entry, false, nil
	}
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT domain, tunnel_id, status, cert_state, last_error, created_at, updated_at
		FROM custom_domains WHERE domain = ?`, cleanDomain).
		Scan(&entry.Domain, &entry.TunnelID, &entry.Status, &entry.CertState, &entry.LastError, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return entry, false, nil
	}
	if err != nil {
		return entry, false, err
	}
	if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
		entry.CreatedAt = parsed
	}
	if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		entry.UpdatedAt = parsed
	}
	return entry, true, nil
}

func (s *Store) ListCustomDomains(limit int) ([]CustomDomain, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT domain, tunnel_id, status, cert_state, last_error, created_at, updated_at
		FROM custom_domains ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CustomDomain
	for rows.Next() {
		var entry CustomDomain
		var createdAt, updatedAt string
		if err := rows.Scan(&entry.Domain, &entry.TunnelID, &entry.Status, &entry.CertState, &entry.LastError, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			entry.CreatedAt = parsed
		}
		if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			entry.UpdatedAt = parsed
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (s *Store) ListHTTPReservations() ([]HTTPReservation, error) {
	rows, err := s.db.Query(`SELECT s.subdomain, s.tunnel_id, IFNULL(GROUP_CONCAT(a.cidr), '')
		FROM subdomains s
		LEFT JOIN ip_allowlists a ON a.tunnel_id = s.tunnel_id
		GROUP BY s.subdomain, s.tunnel_id
		ORDER BY s.subdomain ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []HTTPReservation
	for rows.Next() {
		var subdomain, tunnelID, allowlist string
		if err := rows.Scan(&subdomain, &tunnelID, &allowlist); err != nil {
			return nil, err
		}
		var allowlistValues []string
		if allowlist != "" {
			allowlistValues = strings.Split(allowlist, ",")
		}
		results = append(results, HTTPReservation{Subdomain: subdomain, TunnelID: tunnelID, Allowlist: allowlistValues})
	}
	return results, rows.Err()
}

func (s *Store) InsertLog(entry LogEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if entry.Kind == "" {
		entry.Kind = "http"
	}
	_, err := s.db.Exec(`INSERT INTO logs (tunnel_id, ts, kind, remote_addr, summary, status_code, bytes_in, bytes_out)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, entry.TunnelID, entry.Timestamp.UTC().Format(time.RFC3339), entry.Kind, entry.RemoteAddr, entry.Summary, entry.Status, entry.BytesIn, entry.BytesOut)
	return err
}

func (s *Store) AddMetric(tunnelID string, ts time.Time, reqCount, connCount, bytesIn, bytesOut int64) error {
	if tunnelID == "" {
		return nil
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	minuteBucket := ts.Unix() / 60
	_, err := s.db.Exec(`INSERT INTO metrics_rollup (tunnel_id, minute_bucket, req_count, conn_count, bytes_in, bytes_out)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tunnel_id, minute_bucket) DO UPDATE SET
			req_count = req_count + excluded.req_count,
			conn_count = conn_count + excluded.conn_count,
			bytes_in = bytes_in + excluded.bytes_in,
			bytes_out = bytes_out + excluded.bytes_out`, tunnelID, minuteBucket, reqCount, connCount, bytesIn, bytesOut)
	return err
}

func (s *Store) ListLogs(limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT tunnel_id, ts, kind, remote_addr, summary, status_code, bytes_in, bytes_out
		FROM logs ORDER BY ts DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LogEntry
	for rows.Next() {
		var entry LogEntry
		var ts string
		if err := rows.Scan(&entry.TunnelID, &ts, &entry.Kind, &entry.RemoteAddr, &entry.Summary, &entry.Status, &entry.BytesIn, &entry.BytesOut); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = parsed
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (s *Store) ListMetrics(limit int) ([]MetricRollup, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT tunnel_id, minute_bucket, req_count, conn_count, bytes_in, bytes_out
		FROM metrics_rollup ORDER BY minute_bucket DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MetricRollup
	for rows.Next() {
		var entry MetricRollup
		if err := rows.Scan(&entry.TunnelID, &entry.MinuteBucket, &entry.ReqCount, &entry.ConnCount, &entry.BytesIn, &entry.BytesOut); err != nil {
			return nil, err
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (s *Store) ApplyMigrations(dir string) error {
	if dir == "" {
		return fmt.Errorf("migration dir required")
	}
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	for _, file := range files {
		applied, err := s.isMigrationApplied(file)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		contents, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return err
		}

		if _, err := s.db.Exec(string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", file, err)
		}
		if _, err := s.db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", file, nowUTC()); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) isMigrationApplied(version string) (bool, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(1) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
