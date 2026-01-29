# PortOpener Documentation

Primary reference: [`docs/chat.md`](chat.md)

## Quick Links

- **[VPS Deployment Guide](vps-deployment.md)** — Complete deployment instructions for Ubuntu/Debian VPS with Docker Compose, Caddy, and Cloudflare DNS
- **[Planning Package](../plans/planning-package.md)** — Architecture, roadmap, and implementation milestones
- **[AGENTS.md](../AGENTS.md)** — Development conventions and constraints

## Documentation Index

### Operational Guides

- [`VPS Deployment Guide`](vps-deployment.md) — End-to-end VPS setup, firewall rules, DNS configuration, secrets management, token rotation, rollout/rollback, monitoring, and troubleshooting

### Project Planning

- [`Planning Package`](../plans/planning-package.md) — Complete planning documentation
- [`Executive Summary`](../plans/executive-summary.md) — Project overview and goals
- [`Project Charter`](../plans/project-charter.md) — Scope, stakeholders, and success criteria
- [`Technical Architecture`](../plans/technical-architecture.md) — System design and component architecture
- [`Roadmap`](../plans/roadmap.md) — Implementation milestones and timeline
- [`Risk Assessment`](../plans/risk-assessment.md) — Identified risks and mitigation strategies

### Technical Reference

- [`chat.md`](chat.md) — Project intent, non-negotiables, and feature requirements
- [`relay-protocol.md`](relay-protocol.md) — Tunnel relay protocol specification
- [`techstack.md`](techstack.md) — Technology stack and tooling choices

### Deployment Artifacts

- [`../deploy/Caddyfile`](../deploy/Caddyfile) — Caddy reverse proxy configuration
- [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) — Docker Compose service definitions

---

## What this repo is

This repository contains the implementation of PortOpener, a **personal-use** tunneling system that replicates the core feature set of Port Buddy.

### Key Features

- HTTP(S) tunnels (including WebSockets)
- TCP and UDP tunnels
- Static subdomains under `*.tunnel.<base-domain>`
- Reserved external ports (range 20000–40000)
- Custom domains with automatic TLS via Let's Encrypt
- Per-tunnel IP allowlist
- Token rotation
- Persistence across restarts (SQLite)
- Access logs and basic metrics
- CLI multi-tunnel support with daemon/background mode
- Minimal admin UI

### Architecture

- **Server**: Go (API, router, relay, persistence)
- **CLI**: Go (multi-tunnel + daemon)
- **Web UI**: Static assets served by Go server
- **Edge TLS/Router**: Caddy
- **DNS Provider**: Cloudflare (DNS-01 for ACME)
- **Deployment**: Docker Compose on VPS
- **Persistence**: SQLite

### Development Conventions

- Single-operator system (no multi-tenant or team features)
- Explicit, bounded retention (logs 14 days, metrics rollups 60 days)
- Security-sensitive paths are first-class:
  - Token handling and hashing
  - On-demand TLS gating (`ask` endpoint) is strict and auditable
  - Admin endpoints enforce token auth and admin IP allowlist
- All persistence goes through a small storage layer that owns SQLite access
- Migrations are required for any schema changes

### Getting Started

1. **For deployment**: See [`VPS Deployment Guide`](vps-deployment.md) for complete setup instructions
2. **For development**: Review [`AGENTS.md`](../AGENTS.md) for development workflow and conventions
3. **For understanding**: Read [`chat.md`](chat.md) for project intent and [`technical-architecture.md`](../plans/technical-architecture.md) for system design

## Custom domains + TLS (Phase 5)

Custom domains are mapped via the admin API and protected by Caddy on-demand TLS. Caddy calls the server `ask` endpoint to decide whether issuance is permitted.

### Configure Caddy (Caddyfile)

Use the `on_demand` block to gate issuance through the server:

```
{
  admin off
}

tls {
  on_demand {
    ask http://server:8080/api/tls/ask
  }
}

https:// {
  reverse_proxy server:8080
}
```

### DNS-01 (Cloudflare)

For production, configure the Cloudflare DNS-01 plugin and set credentials:

- `CLOUDFLARE_API_TOKEN` with DNS-edit permissions.
- Use `tls { dns cloudflare {env.CLOUDFLARE_API_TOKEN} }` in the Caddyfile.

### Admin API

- `POST /api/domains` to create/update a mapping.
- `GET /api/domains` to list mappings.
- `GET /api/tls/ask?domain=example.com` is called by Caddy.

Mappings must have `status=enabled` and `tunnel_id` set to allow issuance.
