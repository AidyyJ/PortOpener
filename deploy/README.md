# Deployment Artifacts

This directory contains Docker Compose and Caddy configuration files for deploying PortOpener on a VPS.

> **For complete deployment instructions**, see [`docs/vps-deployment.md`](../docs/vps-deployment.md).

## Files

| File | Purpose |
|------|---------|
| [`docker-compose.yml`](docker-compose.yml) | Docker Compose service definitions for server and Caddy |
| [`Caddyfile`](Caddyfile) | Caddy reverse proxy configuration with on-demand TLS |
| [`caddy.Dockerfile`](caddy.Dockerfile) | Custom Caddy build with Cloudflare DNS-01 module |

## Quick Start

1. **Configure environment variables**:
   ```bash
   cd deploy
   cp .env.example .env
   nano .env  # Edit with your values
   ```

   See [`.env.example`](.env.example) for all required variables and their descriptions.

2. **Build images** (server + custom Caddy):
   ```bash
   sudo docker compose build
   ```

3. **Start services**:
   ```bash
   sudo docker compose up -d
   ```

4. **Verify deployment**:
   ```bash
   sudo docker compose ps
   curl http://localhost:8080/healthz
   ```

## Environment Variables

See [`docs/vps-deployment.md#secrets-and-environment-variables`](../docs/vps-deployment.md#secrets-and-environment-variables) for a complete reference.

## Common Commands

```bash
# Start services
sudo docker compose up -d

# Stop services
sudo docker compose down

# View logs
sudo docker compose logs -f

# Restart services
sudo docker compose restart

# Rebuild after code changes
sudo docker compose build
sudo docker compose up -d

# Backup database
sudo docker run --rm -v portopener_portopener_data:/data -v $(pwd):/backup alpine tar czf /backup/portopener-db-backup-$(date +%Y%m%d).tar.gz /data/portopener.db
```

## Security Notes

- Never commit `.env` to version control
- Use a single shared token (`PORTOPENER_RELAY_TOKEN`) for admin API + relay
- Restrict `PORTOPENER_ADMIN_ALLOWLIST` to trusted IPs
- Keep Cloudflare API token secure
- Use "DNS only" (gray cloud) for Cloudflare DNS records

## Troubleshooting

For common issues and solutions, see [`docs/vps-deployment.md#troubleshooting`](../docs/vps-deployment.md#troubleshooting).
