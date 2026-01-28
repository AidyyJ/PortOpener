# Executive Summary — PortOpener

Authoritative scope source: [`docs/chat.md`](../docs/chat.md)

## What PortOpener is

PortOpener is a **personal-use tunneling system** (not a multi-tenant SaaS) that exposes local services to the public internet via a UK VPS “edge”. It targets the core Port Buddy-like experience defined in [`docs/chat.md`](../docs/chat.md):

- HTTP(S) tunnels including WebSockets
- TCP tunnels
- UDP tunnels
- Static subdomains under `tunnel.<base-domain>`
- Reserved external ports for TCP/UDP
- Custom domains with automatic TLS via Let’s Encrypt
- Per-tunnel IP allowlist
- Token rotation
- Persistence across restarts (SQLite)
- Access logs and basic metrics
- CLI multi-tunnel + daemon/background mode
- Minimal admin UI

## Key decisions (constraints)

- **Stack**: Go server + Go CLI; admin UI served as static assets by the Go server
- **Edge**: Caddy for TLS termination and host-based routing
- **DNS/ACME**: Cloudflare + DNS-01; custom domains use **on-demand TLS** gated by a strict `ask` endpoint
- **Deployment**: Docker Compose on the VPS
- **Public ports**: `80/443` + TCP/UDP range `20000–40000`
- **Admin security**: operator token auth + admin IP allowlist
- **Retention**: access logs 14 days; metrics rollups 60 days
- **CLI OS targets**: Windows and Linux (Ubuntu/Debian compatible)

## What success looks like

- A tunnel can be created from the CLI and remains stable across reconnects and server restarts.
- Static subdomains and reserved ports survive restarts and route reliably.
- Custom domains can be mapped and receive valid TLS automatically, without allowing arbitrary certificate issuance.
- Admin UI can list/kill tunnels, manage reservations and custom domains, rotate token, and view logs/metrics.

## Next deliverable

Implementation scaffolding aligned to [`plans/planning-package.md`](planning-package.md): repository structure, Docker Compose baseline, Caddy config, initial Go server/CLI skeleton, migrations framework, and a minimal admin UI stub.

