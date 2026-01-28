# AGENTS.md

Authoritative constraints source: [`docs/chat.md`](docs/chat.md)

This repository is currently in the planning stage. Implementation should follow the architecture and roadmap in [`plans/planning-package.md`](plans/planning-package.md).

Resolved decisions (treated as constraints):

- CLI OS targets: Windows, Linux (Ubuntu/Debian compatible)
- VPS deployment: Docker Compose
- Retention: logs 14 days; metrics rollups 60 days

---

## Project intent (non-negotiables)

PortOpener is a **personal-use** tunneling system (not multi-tenant SaaS) that must support:

- HTTP(S) tunnels including WebSockets
- TCP tunnels
- UDP tunnels
- Static subdomains under `tunnel.<base-domain>`
- Reserved external ports for TCP/UDP (range `20000–40000`)
- Custom domains with automatic TLS via Let’s Encrypt
- Per-tunnel IP allowlist
- Token rotation
- Persistence across restarts (SQLite)
- Access logs and basic metrics
- CLI multi-tunnel + daemon/background mode
- Minimal admin UI (served as static assets by the Go server)

Reference: [`docs/chat.md`](docs/chat.md)

---

## Chosen stack (planning decision)

- Server: Go
- CLI: Go
- Web UI: static assets served by Go server
- Edge TLS/router: Caddy
- DNS provider: Cloudflare (DNS-01 for ACME; on-demand TLS gated by `ask` endpoint)
- Deployment: Docker Compose on VPS
- Persistence: SQLite

---

## Repository structure (target)

Planned layout (to be created in implementation phase):

- `server/` Go server (API, router, relay, persistence)
- `cli/` Go CLI (multi-tunnel + daemon)
- `web/` static admin UI assets
- `deploy/` Caddyfile, compose files, ops docs
- `migrations/` SQLite migration files
- `docs/` operator documentation
- `plans/` planning artifacts

---

## Development workflow (target)

Until the implementation scaffold exists, this section is informational; commands will be finalized once Go modules and Compose are added.

### Local development goals

- One command starts the dev stack (Caddy + server + local DB).
- One command runs tests.
- One command builds release artifacts for Windows/Linux.

### Conventions

- Keep the system single-operator: do not add multi-user or team abstractions.
- Prefer explicit, bounded retention for logs/metrics.
- Treat security-sensitive paths as first-class:
  - token handling and hashing
  - on-demand TLS gating (`ask` endpoint) should be strict and auditable
  - admin endpoints must enforce token auth and admin IP allowlist

### Data access

- All persistence goes through a small storage layer that owns SQLite access.
- Migrations are required for any schema changes.

### Testing

- Unit tests for allocation/validation logic.
- Integration tests for tunnel setup and proxy flows.

---

## Change management

- Planning changes should update [`plans/planning-package.md`](plans/planning-package.md).
- Implementation changes should stay aligned to the milestones described there.
