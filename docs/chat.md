# chat.md — Personal “Port Buddy”-like Tunnel App (Codex Import)

## Goal
Build a **personal-use** tunneling application (no teams/multi-tenant SaaS features) that replicates the **core feature set** of Port Buddy:
- Public HTTPS URLs exposing local services
- Support for **HTTP(S)** (incl. WebSockets), **TCP**, and **UDP**
- **Static subdomains**, **reserved external ports**
- **Custom domains** with **automatic TLS (Let’s Encrypt)**
- Per-tunnel **IP allowlist**
- **Token rotation**
- Persistence for reservations/mappings
- **Access logs** and **basic metrics**
- CLI can run **multiple tunnels** and support **daemon/background mode**
- **Minimal Web UI** for management

> Passcode-protected tunnels are **optional** (not required for initial scope).

---

## Inputs / references
- Inspired by Port Buddy: https://portbuddy.dev/
- Source repo referenced: https://github.com/amak-tech/port-buddy

---

## Non-negotiable scope (Yes/No decisions already made)

### Protocol support
- ✅ HTTP(S) tunnels (implied core)
- ✅ TCP tunnels
- ✅ UDP tunnels

### Addressing
- ✅ Static subdomains (e.g., `myapp.tunnel.example.com`)
- ✅ Reserved external ports for TCP and UDP

### Custom domains & TLS
- ✅ Custom domains (bring-your-own domain)
- ✅ Automatic TLS for custom domains via ACME/Let’s Encrypt
- ❌ Manual “reverse proxy in front” mode (you manage TLS) — NOT desired

### Security
- ✅ Per-tunnel IP allowlist
- ✅ Token rotation (invalidate/reissue CLI auth)
- ⚪ Passcode tunnels — OPTIONAL (can be later)

### Persistence (must survive server restarts)
- ✅ Static subdomain reservations
- ✅ Reserved port allocations
- ✅ Custom domain -> tunnel mappings

### Observability
- ✅ Per-tunnel access logs
- ✅ Basic metrics (requests/connections + bytes in/out)

### Client experience
- ✅ One CLI process can run multiple tunnels at once
- ✅ Daemon/background mode
- ❌ Config profiles (not needed)

### Admin surface
- ✅ Minimal web UI (list/kill tunnels, manage reservations/domains, rotate token)

---

## Deployment / hosting decision
You want the tunnel “edge” on a UK VPS with strong networking (unmetered 1Gbps acceptable).

Chosen starting VPS:
- Provider: **OVHcloud (UK region)**
- Plan link: https://www.ovhcloud.com/en-gb/vps/configurator/?planCode=vps-2025-model2&brick=VPS%2BModel%2B2&pricing=upfront12&processor=%20&vcore=6__vCore&storage=100__SSD__NVMe
- Key constraints you care about:
  - UK-based
  - 1Gbps unmetered OK
  - Must support UDP and opening port ranges (for reserved ports)
  - Public IPv4
  - Wildcard DNS support for `*.tunnel.<your-domain>`

---

## Target system shape (personal-only, but capable)

### Server (single deploy on VPS)
One server deployment providing:
1) **Gateway / Router**
   - Terminates TLS for HTTP(S)
   - Routes by Host header:
     - `https://<subdomain>.tunnel.<base-domain>` -> tunnel session
     - `https://<custom-domain>` -> mapped tunnel session
   - Supports WebSocket upgrades for HTTP tunnels

2) **Tunnel Relay (control + data plane)**
   - Persistent outbound connections from clients to server
   - Multiplexing for:
     - HTTP request/response (including WS)
     - TCP streams
     - UDP datagrams

3) **Port listeners**
   - Allocated and reserved external TCP ports -> forward to client target
   - Allocated and reserved external UDP ports -> datagram relay to client target

4) **ACME/TLS**
   - Automatic cert issuance/renewal for custom domains (Let’s Encrypt)
   - Domain verification (DNS-based recommended)

5) **API + Minimal Admin UI**
   - List active tunnels
   - Kill tunnels
   - Reserve/release subdomains
   - Reserve/release ports
   - Add/manage custom domains (show required DNS record; status)
   - Rotate token (display new token once)
   - View logs + basic metrics

6) **Persistence**
   - SQLite (or equivalent single-node DB) on VPS

---

## Client (CLI)
Key behaviors:
- `init <token>` stores token locally
- Start tunnels:
  - HTTP: `tool http <localPort> [--domain <name>] [--allow <cidr>]`
  - TCP: `tool tcp <localPort> [--reserve-port <externalPort>]`
  - UDP: `tool udp <localPort> [--reserve-port <externalPort>]`
- One CLI process can run **multiple tunnels** concurrently
- **Daemon mode**:
  - `tool daemon start` / `tool daemon stop`
  - Auto-reconnect
- Status output includes:
  - Public URLs for HTTP
  - Host:port for TCP/UDP
  - Allowlist info (optional)
- Optional later:
  - `--passcode` for HTTP tunnels

---

## Persistence / DB (SQLite) suggested tables
- `tokens` (hashed token, created_at, revoked_at)
- `tunnels` (id, name, protocol, local_host, local_port, created_at, last_seen, status)
- `subdomains` (subdomain, tunnel_id, reserved, created_at)
- `port_reservations` (protocol tcp/udp, external_port, tunnel_id, reserved, created_at)
- `custom_domains` (domain, tunnel_id, status, cert_state, last_error, created_at)
- `ip_allowlists` (tunnel_id, cidr)
- `logs` (tunnel_id, ts, kind=http|tcp|udp, remote_addr, summary, status_code, bytes_in, bytes_out)
- `metrics_rollup` (tunnel_id, minute_bucket, req_count, conn_count, bytes_in, bytes_out)

---

## Minimal Web UI pages
1) **Dashboard**
   - Active tunnels list
   - Kill tunnel
2) **Reservations**
   - Manage static subdomains
   - Manage reserved ports (TCP/UDP)
3) **Domains**
   - Add domain, show DNS validation requirements
   - Status: pending/verified/cert issued/error
4) **Security**
   - Rotate token (show once)
5) **Logs/Metrics**
   - Recent access logs per tunnel
   - Basic counters (req/conn, bytes)

No accounts/teams; **single-user token** auth.

---

## Build order (milestones with intent)

### M1 — HTTP tunnels + static subdomains + IP allowlist
- HTTPS listener + wildcard subdomain routing
- WebSocket support
- Per-tunnel allowlist enforcement
- Basic logs + counters

### M2 — Persistence + Minimal Admin UI
- SQLite persistence for tunnels/reservations
- UI for listing/killing tunnels + managing allowlists & subdomains

### M3 — TCP tunnels + reserved ports
- Port allocator + reserved port feature
- Multi-connection support
- Logs: connection start/stop + bytes

### M4 — UDP tunnels + reserved ports
- Datagram relay + session mapping + timeouts
- Metrics: datagrams + bytes

### M5 — Custom domains + Auto TLS
- DNS-based verification
- ACME cert issuance + renewal
- Map custom domain -> tunnel
- UI status & error reporting

### M6 — Multi-tunnel CLI + Daemon mode
- Run multiple tunnels in one CLI session
- Background mode + auto-reconnect
- Server cleanup for dropped clients

### M7 — Token rotation
- Revoke old token, issue new token
- UI action for rotation
- Client re-init flow

---

## Ops/networking assumptions to plan for
- Base domain: `tunnel.<your-domain>`
- Wildcard DNS: `*.tunnel.<your-domain>` -> VPS IP
- Expose:
  - 80/443 for HTTP(S) + ACME challenges (or DNS-01 exclusively)
  - A configurable port range for TCP/UDP reserved/allocated ports (e.g., 20000–40000)
- Firewall:
  - Allow 80/443 inbound
  - Allow chosen TCP/UDP port range inbound
  - Lock admin API/UI behind token auth

---

## Notes about Plex usage
- Plex could be a large bandwidth driver; VPS choice (unmetered 1Gbps) helps.
- Keep an eye on throughput and concurrent streams; CPU matters mainly for transcoding.

---

## Deliverable expectation
Codex should generate:
- A repo scaffold for server + CLI + minimal web UI
- A multiplexing protocol between client and server for HTTP/TCP/UDP
- SQLite persistence + migrations
- ACME automation for custom domains
- CLI daemonization + multi-tunnel orchestration
- Logging + metrics
- Deployment docs for OVH VPS + DNS + firewall rules
