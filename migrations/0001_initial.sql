PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  revoked_at TEXT
);

CREATE TABLE IF NOT EXISTS tunnels (
  id TEXT PRIMARY KEY,
  name TEXT,
  protocol TEXT NOT NULL,
  local_host TEXT NOT NULL,
  local_port INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  last_seen TEXT
);

CREATE TABLE IF NOT EXISTS subdomains (
  subdomain TEXT PRIMARY KEY,
  tunnel_id TEXT,
  reserved INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  FOREIGN KEY (tunnel_id) REFERENCES tunnels(id)
);

CREATE TABLE IF NOT EXISTS port_reservations (
  protocol TEXT NOT NULL,
  external_port INTEGER NOT NULL,
  tunnel_id TEXT,
  reserved INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  PRIMARY KEY (protocol, external_port),
  FOREIGN KEY (tunnel_id) REFERENCES tunnels(id)
);

CREATE TABLE IF NOT EXISTS custom_domains (
  domain TEXT PRIMARY KEY,
  tunnel_id TEXT,
  status TEXT NOT NULL,
  cert_state TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT,
  FOREIGN KEY (tunnel_id) REFERENCES tunnels(id)
);

CREATE TABLE IF NOT EXISTS ip_allowlists (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tunnel_id TEXT NOT NULL,
  cidr TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (tunnel_id, cidr),
  FOREIGN KEY (tunnel_id) REFERENCES tunnels(id)
);

CREATE TABLE IF NOT EXISTS logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tunnel_id TEXT,
  ts TEXT NOT NULL,
  kind TEXT NOT NULL,
  remote_addr TEXT,
  summary TEXT,
  status_code INTEGER,
  bytes_in INTEGER,
  bytes_out INTEGER,
  FOREIGN KEY (tunnel_id) REFERENCES tunnels(id)
);

CREATE TABLE IF NOT EXISTS metrics_rollup (
  tunnel_id TEXT NOT NULL,
  minute_bucket INTEGER NOT NULL,
  req_count INTEGER NOT NULL DEFAULT 0,
  conn_count INTEGER NOT NULL DEFAULT 0,
  bytes_in INTEGER NOT NULL DEFAULT 0,
  bytes_out INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (tunnel_id, minute_bucket),
  FOREIGN KEY (tunnel_id) REFERENCES tunnels(id)
);

CREATE INDEX IF NOT EXISTS idx_tunnels_status ON tunnels(status);
CREATE INDEX IF NOT EXISTS idx_subdomains_tunnel ON subdomains(tunnel_id);
CREATE INDEX IF NOT EXISTS idx_ports_tunnel ON port_reservations(tunnel_id);
CREATE INDEX IF NOT EXISTS idx_domains_status ON custom_domains(status);
CREATE INDEX IF NOT EXISTS idx_logs_tunnel_ts ON logs(tunnel_id, ts);

