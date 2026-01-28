# Roadmap — PortOpener

Authoritative scope source: [`docs/chat.md`](../docs/chat.md)

This roadmap preserves the milestone intent from [`docs/chat.md`](../docs/chat.md) while incorporating the resolved constraints:

- Go + Caddy + Cloudflare DNS-01
- Docker Compose deployment
- CLI targets Windows/Linux (Ubuntu/Debian compatible)
- Retention: logs 14 days; metrics rollups 60 days

## Phase 0 — Scaffold and baseline operations

Deliverables:

- Repository structure scaffold (`server/`, `cli/`, `web/`, `deploy/`, `migrations/`)
- Docker Compose baseline with Caddy + server
- Migrations framework and initial schema
- Local dev workflow and CI skeleton

Acceptance:

- `docker compose up` brings up edge + server locally.
- Admin UI placeholder served.

## Phase 1 (M1) — HTTP tunnels + static subdomains + IP allowlist

Deliverables:

- HTTP tunnel registration
- Wildcard subdomain routing via Caddy
- WebSocket support
- Per-tunnel IP allowlist enforcement
- Basic logging + counters

Acceptance:

- HTTP and WebSocket traffic works end-to-end with allowlist enforcement.

## Phase 2 (M2) — Persistence + minimal admin UI

Deliverables:

- SQLite persistence for tunnels and reservations
- Admin UI: list/kill tunnels; manage subdomains and allowlists
- Logs/metrics views

Acceptance:

- Server restart preserves reservations and the UI can manage them.

## Phase 3 (M3) — TCP tunnels + reserved ports

Deliverables:

- TCP port allocator and reservation
- Persistence for port reservations
- Multi-connection support
- Connection logs and byte counters

Acceptance:

- Reserved TCP ports forward reliably across reconnects.

## Phase 4 (M4) — UDP tunnels + reserved ports

Deliverables:

- UDP relay with session mapping
- Timeouts/cleanup
- Datagram/byte metrics

Acceptance:

- UDP forwarding works for a test service with documented limitations.

## Phase 5 (M5) — Custom domains + automatic TLS

Deliverables:

- Custom domain mapping UI and persistence
- On-demand TLS gated by `ask`
- DNS-01 ACME via Cloudflare
- Status and error reporting in UI

Acceptance:

- Mapped custom domain obtains valid TLS and routes; unmapped domain cannot trigger issuance.

## Phase 6 (M6) — Multi-tunnel CLI + daemon mode

Deliverables:

- Multiple tunnels in one CLI process
- Daemon/background mode for Windows/Linux
- Auto-reconnect; server cleanup for dropped clients

Acceptance:

- Tunnels survive brief network interruptions and can be managed in daemon mode.

## Phase 7 (M7) — Token rotation

Deliverables:

- UI action to rotate token (display once)
- Server-side revocation
- CLI re-init flow

Acceptance:

- Old token is invalidated and new token works as intended.

