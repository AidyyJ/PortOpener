# Tech Stack

## Core

- **Server** — Go
- **CLI** — Go (Windows, Linux)
- **Web UI** — Static assets served by Go server

## Edge & TLS

- **Caddy** — TLS termination, reverse proxy, on-demand TLS
- **Let's Encrypt** — Automatic certificates
- **Cloudflare** — DNS provider (DNS-01 ACME)

## Deployment

- **Docker Compose** — VPS deployment
- **SQLite** — Persistence layer

## Networking

- **Protocols** — HTTP(S) with WebSockets, TCP, UDP
- **Port range** — 20000–40000 (public TCP/UDP tunnels)
