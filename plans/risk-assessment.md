# Risk Assessment — PortOpener

Authoritative scope source: [`docs/chat.md`](../docs/chat.md)

## Resource needs

This project can be completed by a single engineer, but spans:

- Go networking (TCP/UDP relay, backpressure, concurrency)
- HTTP proxy semantics (streaming, headers, WebSockets)
- Caddy/ACME operations (DNS-01, on-demand TLS gating)
- VPS operations (firewall, Docker Compose, backups)
- Minimal web UI wiring

Optional: independent security review of token handling and on-demand TLS gating.

## Risk register

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| On-demand TLS abuse (unauthorized cert issuance) | High | Medium | Strict `ask` gate backed by SQLite; explicit enable state; log all decisions; consider rate limiting `ask`. |
| UDP complexity (session mapping, NAT churn, timeouts) | Medium | Medium | Define clear timeouts; bound session tables; expose session metrics; document limitations. |
| Docker networking for large port range | Medium | Medium | Use host networking on VPS; document port ownership and firewall rules. |
| Log/metrics storage growth | Medium | Medium | Enforce retention (14d logs, 60d rollups); index; background cleanup; avoid high-cardinality dimensions. |
| Reliability on reconnect (stale sessions, half-open streams) | Medium | Medium | Heartbeats/leases; server-side cleanup; idempotent registration and reservations. |
| Admin surface exposure | High | Low | Token auth + admin IP allowlist; minimal endpoints; audit logging; CSRF-safe design for browser flows. |
| Operational secrets handling (Cloudflare API token) | High | Medium | Store via Compose secrets/env; least privilege; rotate periodically; never log secrets. |

## Bottlenecks and early spikes

Recommended early spikes to reduce uncertainty:

- Validate Caddy DNS-01 + on-demand TLS `ask` gating in a minimal environment.
- Validate relay transport choice can handle HTTP streaming + WebSockets.
- Validate UDP relay framing and session mapping on a simple echo service.

