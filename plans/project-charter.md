# Project Charter — PortOpener

Authoritative scope source: [`docs/chat.md`](../docs/chat.md)

## Vision

Provide a reliable personal tunnel “edge” that can securely expose local services over HTTP(S)/TCP/UDP through stable public addresses (static subdomains and reserved ports), with minimal operational burden.

## Problem statement

Existing options tend to be SaaS/team-oriented, lack UDP support, or complicate safe custom-domain TLS. PortOpener will deliver the required feature set for a **single operator** on a UK VPS.

## Scope

### In scope (non-negotiable)

From [`docs/chat.md`](../docs/chat.md):

- Protocols: HTTP(S) including WebSockets; TCP; UDP
- Addressing: static subdomains; reserved external ports (TCP/UDP)
- Custom domains with automatic TLS (Let’s Encrypt via ACME)
- Per-tunnel IP allowlist
- Token rotation
- Persistence across restarts (SQLite)
- Access logs and basic metrics
- CLI can run multiple tunnels; daemon/background mode
- Minimal web UI for management

### Out of scope (initially)

- Multi-tenant SaaS: accounts, teams, roles, billing
- Config profiles
- Mandatory passcode-protected tunnels (explicitly optional)
- Advanced observability (distributed tracing, high-cardinality analytics)

## Stakeholders

- Primary: single operator
- Secondary: none

## Success criteria

- End-to-end tunnel flows work for HTTP(S)/WS, TCP, and UDP.
- Reservations (subdomains, ports) are persisted and enforced.
- Custom domains can be mapped and obtain valid TLS automatically; unmapped domains cannot trigger issuance.
- Admin UI can perform essential operations and display logs/metrics.

## Constraints and assumptions

- UK VPS with public IPv4 and ability to open inbound UDP and large port ranges.
- Cloudflare is the DNS provider for the base domain.
- Deployment uses Docker Compose.
- Single-user token auth with an admin IP allowlist.
- Retention: access logs 14 days; metrics rollups 60 days.

