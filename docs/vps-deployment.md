# VPS Deployment Guide

This guide provides end-to-end instructions for deploying PortOpener on an Ubuntu/Debian VPS using Docker Compose, Caddy, and Cloudflare DNS.

> **Prerequisites**: Read [`docs/chat.md`](chat.md:1) for project intent and [`AGENTS.md`](../AGENTS.md:1) for architectural constraints.

---

## Table of Contents

1. [VPS Prerequisites](#vps-prerequisites)
2. [Firewall Configuration](#firewall-configuration)
3. [Cloudflare DNS Setup](#cloudflare-dns-setup)
4. [Docker Compose Deployment](#docker-compose-deployment)
5. [Caddy Configuration](#caddy-configuration)
6. [Secrets and Environment Variables](#secrets-and-environment-variables)
7. [Token Rotation](#token-rotation)
8. [Rollout and Rollback](#rollout-and-rollback)
9. [Monitoring and Retention](#monitoring-and-retention)
10. [Troubleshooting](#troubleshooting)

---

## VPS Prerequisites

### Supported OS

- **Ubuntu 20.04 LTS or later**
- **Debian 11 (Bullseye) or later**

### System Requirements

- **CPU**: 2+ cores recommended
- **RAM**: 2GB minimum, 4GB recommended
- **Storage**: 20GB minimum (for Docker images, logs, and database)
- **Network**: 1Gbps unmetered (recommended for high-throughput use cases like Plex)

### Install Docker and Docker Compose Plugin

Run these commands as root or with sudo:

```bash
# Update package index
sudo apt-get update

# Install prerequisites
sudo apt-get install -y ca-certificates curl gnupg lsb-release

# Add Docker's official GPG key
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Set up Docker repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Update package index again
sudo apt-get update

# Install Docker Engine, Docker Compose plugin, and containerd
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# Verify installation
sudo docker version
sudo docker compose version
```

### Enable Docker Service

```bash
sudo systemctl enable docker
sudo systemctl start docker
```

### Add User to Docker Group (Optional)

If you want to run Docker without sudo:

```bash
sudo usermod -aG docker $USER
newgrp docker
```

---

## Firewall Configuration

PortOpener requires specific firewall rules to operate correctly. Configure your VPS firewall (UFW, iptables, or cloud provider firewall) with the following rules.

### Required Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 80 | TCP | HTTP (ACME HTTP-01 challenges) |
| 443 | TCP | HTTPS (tunnel traffic + ACME) |
| 20000-40000 | TCP | Reserved TCP tunnel ports |
| 20000-40000 | UDP | Reserved UDP tunnel ports |

### UFW Configuration

```bash
# Enable UFW if not already enabled
sudo ufw --force enable

# Allow SSH (adjust port if you use a non-default SSH port)
sudo ufw allow 22/tcp

# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Allow TCP port range for tunnels
sudo ufw allow 20000:40000/tcp

# Allow UDP port range for tunnels
sudo ufw allow 20000:40000/udp

# Check status
sudo ufw status verbose
```

### Cloud Provider Firewall

If your VPS provider has an additional firewall (e.g., OVHcloud, DigitalOcean, AWS Security Groups), configure it with the same rules:

- **Inbound**: Allow TCP 22, 80, 443, and TCP/UDP 20000-40000 from `0.0.0.0/0`
- **Outbound**: Allow all traffic (required for Docker and ACME)

### Admin Surface Restrictions

The admin API and UI are protected by:

1. **Token authentication** via `Authorization` or `X-Admin-Token` headers (see [`server/internal/admin/http.go`](../server/internal/admin/http.go:33))
2. **IP allowlist** via `PORTOPENER_ADMIN_ALLOWLIST` environment variable

**Important**: Always set `PORTOPENER_ADMIN_ALLOWLIST` to restrict admin access to trusted IP addresses. If you leave it empty, the admin API will be blocked.

If you run PortOpener behind a reverse proxy that sets `X-Forwarded-For`, the server will use the **first** IP in that header for allowlist checks. Ensure your proxy is trusted and strips untrusted `X-Forwarded-For` headers from the public edge.

---

## Cloudflare DNS Setup

PortOpener uses Cloudflare DNS for wildcard subdomains and DNS-01 ACME challenges.

### Required DNS Records

Assuming your base domain is `example.com`, create these records:

| Type | Name | Value | Proxy |
|------|------|-------|-------|
| A | `tunnel` | `<your-vps-ip>` | **DNS only** (gray cloud) |
| A | `admin.tunnel` | `<your-vps-ip>` | **DNS only** (gray cloud) |

The wildcard `*.tunnel.example.com` is automatically covered by the `tunnel` A record.

### Why "DNS Only" (Gray Cloud)?

Cloudflare Proxy (orange cloud) would interfere with:
- TCP/UDP tunnel traffic on ports 20000-40000
- Direct WebSocket connections
- Custom domain TLS termination

Set both records to **DNS only** (gray cloud icon) to bypass Cloudflare's proxy.

### Custom Domain DNS Records

For custom domains (e.g., `myapp.example.com`), create:

| Type | Name | Value | Proxy |
|------|------|-------|-------|
| CNAME | `myapp` | `tunnel.example.com` | **DNS only** (gray cloud) |

### Cloudflare API Token for DNS-01

PortOpener uses DNS-01 challenges for Let's Encrypt certificate issuance. Create a Cloudflare API token with:

1. Go to **My Profile → API Tokens**
2. Click **Create Token**
3. Use the **Edit zone DNS** template
4. Set permissions:
   - **Zone** → **DNS** → **Edit**
   - Include: **Specific zone** → `example.com`
5. Set TTL to your preference (e.g., 1 hour)
6. Copy the token (you won't see it again)

**Store this token securely** — you'll use it as `CLOUDFLARE_API_TOKEN` in your Caddyfile.

---

## Docker Compose Deployment

### Clone or Upload Repository

```bash
# Clone the repository (replace with your repo URL)
git clone https://github.com/yourusername/PortOpener.git
cd PortOpener
```

Or upload the repository files to your VPS using `scp`, `rsync`, or SFTP.

### Configure Environment Variables

Create a `.env` file in the `deploy/` directory:

```bash
cd deploy
nano .env
```

Add the following variables:

```env
# Server configuration
PORTOPENER_HTTP_ADDR=:8080
PORTOPENER_WEB_ROOT=/app/web
PORTOPENER_DB_PATH=/data/portopener.db
PORTOPENER_MIGRATIONS_DIR=/app/migrations

# Admin token (optional). For a single-token setup, leave blank or set to the same value as PORTOPENER_RELAY_TOKEN.
PORTOPENER_ADMIN_TOKEN=

# Admin IP allowlist (comma-separated CIDR blocks)
PORTOPENER_ADMIN_ALLOWLIST=your-home-ip/32,another-trusted-ip/32

# Relay token (used by CLI clients)
# This is the shared token for admin API + relay.
PORTOPENER_RELAY_TOKEN=your-super-secret-relay-token-here

# Cloudflare API token for DNS-01 challenges
CLOUDFLARE_API_TOKEN=your-cloudflare-api-token-here

# Email for Let's Encrypt
ACME_EMAIL=you@example.com

# Base domain (without tunnel subdomain)
BASE_DOMAIN=example.com
```

**Security Notes**:
- Generate strong, random tokens (minimum 32 characters)
- Never commit `.env` to version control
- Restrict `PORTOPENER_ADMIN_ALLOWLIST` to your trusted IPs
- Use `PORTOPENER_RELAY_TOKEN` as the single shared token (leave `PORTOPENER_ADMIN_TOKEN` empty or set it to the same value)

### Update Caddyfile

Edit [`deploy/Caddyfile`](../deploy/Caddyfile:1) and ensure environment variables are set:

```caddyfile
{
  admin off
  email {env.ACME_EMAIL}
  on_demand_tls {
    ask http://server:8080/api/tls/ask
  }
}

admin.tunnel.{env.BASE_DOMAIN} {
  tls {
    dns cloudflare {env.CLOUDFLARE_API_TOKEN}
  }
  reverse_proxy server:8080
}

*.tunnel.{env.BASE_DOMAIN} {
  tls {
    dns cloudflare {env.CLOUDFLARE_API_TOKEN}
  }
  reverse_proxy server:8080
}

https:// {
  tls {
    dns cloudflare {env.CLOUDFLARE_API_TOKEN}
    on_demand
  }
  reverse_proxy server:8080
}
```

### Update docker-compose.yml

Review [`deploy/docker-compose.yml`](../deploy/docker-compose.yml:1) and ensure it matches your environment:

```yaml
name: portopener

services:
  server:
    build:
      context: ..
      dockerfile: server/Dockerfile
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - portopener_data:/data
    ports:
      - "127.0.0.1:8080:8080"
      - "20000-40000:20000-40000"
      - "20000-40000:20000-40000/udp"

  caddy:
    build:
      context: .
      dockerfile: caddy.Dockerfile
    restart: unless-stopped
    depends_on:
      - server
    ports:
      - "80:80"
      - "443:443"
    env_file:
      - .env
      - "20000-40000:20000-40000"
      - "20000-40000:20000-40000/udp"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config

volumes:
  portopener_data:
  caddy_data:
  caddy_config:
```

**Key Changes**:
- Uses `.env` for configuration values
- Adds port ranges for TCP/UDP tunnels (20000-40000)
- Adds `portopener_data` volume for database persistence
- Uses a custom Caddy image with the Cloudflare DNS module

### Build and Start Services

```bash
# Build images (first time only)
sudo docker compose build

# Start services in detached mode
sudo docker compose up -d

# Check status
sudo docker compose ps

# View logs
sudo docker compose logs -f
```

### Verify Deployment

```bash
# Check server health
curl http://localhost:8080/healthz

# Check admin UI (from allowed IP)
curl -H "Authorization: Bearer your-token" https://admin.tunnel.example.com/api/tunnels
```

---

## Caddy Configuration

### On-Demand TLS with `ask` Gate

PortOpener uses Caddy's on-demand TLS feature with a strict authorization gate. This prevents arbitrary certificate issuance and ensures only approved domains receive certificates.

#### How the `ask` Endpoint Works

When Caddy receives a request for a new domain (e.g., `myapp.example.com`):

1. Caddy calls `http://server:8080/api/tls/ask?domain=myapp.example.com`
2. The server checks if the domain exists in the `custom_domains` table with:
   - `status = 'enabled'`
   - `tunnel_id` is set (not empty)
3. If both conditions are met, the server returns HTTP 200 OK
4. Caddy proceeds with certificate issuance via DNS-01 challenge
5. If the domain is not approved, the server returns HTTP 403 Forbidden
6. Caddy denies certificate issuance

**Reference**: [`server/internal/admin/http.go`](../server/internal/admin/http.go:165)

#### Security Benefits

- **Prevents certificate spam**: Attackers cannot request certificates for random domains
- **Domain verification**: Only domains explicitly added via the admin API can get certificates
- **Tunnel binding**: Certificates are only issued for domains with an active tunnel

#### Testing the `ask` Endpoint

```bash
# Test with an approved domain (after adding it via admin API)
curl "http://localhost:8080/api/tls/ask?domain=myapp.example.com"

# Expected: HTTP 200 OK (empty body)

# Test with an unapproved domain
curl "http://localhost:8080/api/tls/ask?domain=attacker.example.com"

# Expected: HTTP 403 Forbidden
```

### DNS-01 vs HTTP-01 Challenges

PortOpener uses **DNS-01** challenges for ACME certificate issuance:

| Challenge Type | Pros | Cons |
|----------------|------|------|
| **DNS-01** | Works for wildcard certs, no need to open port 80 for each domain | Requires Cloudflare API access |
| **HTTP-01** | Simpler, no DNS API needed | Cannot issue wildcard certs, requires port 80 for each domain |

**Recommendation**: Use DNS-01 for production deployments with Cloudflare.

### Caddy Logs

View Caddy logs to diagnose TLS issues:

```bash
sudo docker compose logs -f caddy
```

Look for:
- Certificate issuance success/failure
- `ask` endpoint responses
- DNS challenge errors

---

## Secrets and Environment Variables

### Environment Variables Reference

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `PORTOPENER_HTTP_ADDR` | Yes | Server listen address | `:8080` |
| `PORTOPENER_WEB_ROOT` | Yes | Path to web UI assets | `/app/web` |
| `PORTOPENER_DB_PATH` | Yes | SQLite database file path | `/data/portopener.db` |
| `PORTOPENER_MIGRATIONS_DIR` | Yes | Path to migration files | `/app/migrations` |
| `PORTOPENER_ADMIN_TOKEN` | No | Optional admin API token (leave blank or set equal to relay token) | `random-32-char-string` |
| `PORTOPENER_ADMIN_ALLOWLIST` | Yes | Comma-separated CIDR blocks for admin access | `1.2.3.4/32,5.6.7.8/32` |
| `PORTOPENER_RELAY_TOKEN` | Yes | Token used by CLI clients and admin API | `random-32-char-string` |
| `CLOUDFLARE_API_TOKEN` | Yes | Cloudflare API token for DNS-01 | `cloudflare-token` |
| `ACME_EMAIL` | Yes | Email for Let's Encrypt notifications | `you@example.com` |
| `BASE_DOMAIN` | Yes | Base domain (without tunnel subdomain) | `example.com` |

### Generating Secure Tokens

Use `openssl` to generate a cryptographically secure token (single token for both relay + admin API):

```bash
openssl rand -base64 32
```

### Database Volume and Persistence

The SQLite database is stored in the `portopener_data` Docker volume:

```bash
# List volumes
sudo docker volume ls

# Inspect volume
sudo docker volume inspect portopener_portopener_data

# Backup database
sudo docker run --rm -v portopener_portopener_data:/data -v $(pwd):/backup alpine tar czf /backup/portopener-db-backup-$(date +%Y%m%d).tar.gz /data/portopener.db

# Restore database
sudo docker run --rm -v portopener_portopener_data:/data -v $(pwd):/backup alpine tar xzf /backup/portopener-db-backup-YYYYMMDD.tar.gz -C /
```

### Backup Strategy

#### Automated Backups

Create a cron job for daily backups:

```bash
# Edit crontab
sudo crontab -e

# Add this line (runs daily at 2 AM)
0 2 * * * cd /path/to/PortOpener/deploy && /usr/bin/docker run --rm -v portopener_portopener_data:/data -v /backups:/backup alpine tar czf /backup/portopener-db-backup-$(date +\%Y\%m\%d).tar.gz /data/portopener.db
```

#### Manual Backup

```bash
# Stop services
sudo docker compose down

# Copy database
sudo docker run --rm -v portopener_portopener_data:/data -v $(pwd):/backup alpine cp /data/portopener.db /backup/

# Restart services
sudo docker compose up -d
```

#### Restore from Backup

```bash
# Stop services
sudo docker compose down

# Restore database
sudo docker run --rm -v portopener_portopener_data:/data -v $(pwd):/backup alpine cp /backup/portopener.db /data/

# Restart services
sudo docker compose up -d
```

---

## Token Rotation

Token rotation invalidates the old token and issues a new one. Use this if you suspect token compromise or as a periodic security measure.

### Rotate Token

**Warning**: After rotating the token, update `PORTOPENER_RELAY_TOKEN` in `.env` and restart the server.

```bash
# Rotate token via API
curl -X POST \
  -H "Authorization: Bearer your-current-token" \
  https://admin.tunnel.example.com/api/token/rotate

# Response:
# {"token":"new-admin-token-here"}
```

**Steps to complete rotation**:

1. Copy the new token from the response
2. Update `PORTOPENER_RELAY_TOKEN` in your `.env` file
3. Restart the server:
   ```bash
   sudo docker compose restart server
   ```
4. Verify access with the new token

### Update CLI Clients

The relay token is used by CLI clients to authenticate with the server and the admin API. After rotation:

1. Rotate the token via admin API (same as above)
2. Update `PORTOPENER_RELAY_TOKEN` in your `.env` file
3. Restart the server
4. Re-initialize all CLI clients:
   ```bash
   portopener init new-relay-token-here
   ```

### Token Rotation Best Practices

- **Store tokens securely**: Use a password manager or encrypted file
- **Rotate regularly**: Every 90 days for production deployments
- **Rotate after compromise**: Immediately if you suspect token leakage
- **Document rotation**: Keep a log of token rotations for audit purposes

---

## Rollout and Rollback

### Safe Deployment Procedure

#### 1. Prepare for Deployment

```bash
# SSH into VPS
ssh user@your-vps-ip

# Navigate to deploy directory
cd /path/to/PortOpener/deploy

# Pull latest changes
git pull origin main

# Check what changed
git log --oneline -5
```

#### 2. Backup Before Deployment

```bash
# Backup database
sudo docker run --rm -v portopener_portopener_data:/data -v $(pwd):/backup alpine tar czf /backup/pre-deploy-$(date +%Y%m%d-%H%M%S).tar.gz /data/portopener.db

# Backup Caddyfile and docker-compose.yml
cp Caddyfile Caddyfile.backup
cp docker-compose.yml docker-compose.yml.backup
```

#### 3. Deploy Changes

```bash
# Rebuild images
sudo docker compose build

# Restart services (zero-downtime if using health checks)
sudo docker compose up -d --force-recreate

# Check logs
sudo docker compose logs -f
```

#### 4. Verify Deployment

```bash
# Check service status
sudo docker compose ps

# Check health endpoint
curl http://localhost:8080/healthz

# Test admin API
curl -H "Authorization: Bearer your-token" https://admin.tunnel.example.com/api/tunnels

# Test tunnel (from CLI client)
portopener http 8080 --domain test
```

### Rollback Procedure

If deployment fails:

```bash
# Stop services
sudo docker compose down

# Restore previous Caddyfile and docker-compose.yml
cp Caddyfile.backup Caddyfile
cp docker-compose.yml.backup docker-compose.yml

# Rebuild and restart
sudo docker compose build
sudo docker compose up -d

# Restore database if needed
sudo docker run --rm -v portopener_portopener_data:/data -v $(pwd):/backup alpine tar xzf /backup/pre-deploy-YYYYMMDD-HHMMSS.tar.gz -C /

# Restart services
sudo docker compose restart server
```

### Zero-Downtime Updates

For zero-downtime updates:

1. **Use rolling updates**: Deploy new containers alongside old ones
2. **Health checks**: Ensure new containers pass health checks before switching traffic
3. **Database migrations**: Run migrations before deploying new code
4. **Graceful shutdown**: Allow existing connections to drain before terminating

**Note**: PortOpener currently uses simple restarts. For production, consider implementing a blue-green deployment strategy.

---

## Monitoring and Retention

### Log Retention

PortOpener retains access logs for **14 days**. Logs are stored in the `logs` table in the SQLite database.

#### View Logs

```bash
# Via API
curl -H "Authorization: Bearer your-token" \
  "https://admin.tunnel.example.com/api/logs?limit=100"

# Via SQLite
sudo docker exec -it portopener-server-1 sqlite3 /data/portopener.db \
  "SELECT * FROM logs ORDER BY ts DESC LIMIT 100;"
```

#### Manual Log Cleanup

If you need to manually clean logs:

```bash
# Delete logs older than 14 days
sudo docker exec -it portopener-server-1 sqlite3 /data/portopener.db \
  "DELETE FROM logs WHERE ts < datetime('now', '-14 days');"
```

### Metrics Retention

PortOpener retains metrics rollups for **60 days**. Metrics are aggregated by minute in the `metrics_rollup` table.

#### View Metrics

```bash
# Via API
curl -H "Authorization: Bearer your-token" \
  "https://admin.tunnel.example.com/api/metrics?limit=100"

# Via SQLite
sudo docker exec -it portopener-server-1 sqlite3 /data/portopener.db \
  "SELECT * FROM metrics_rollup ORDER BY minute_bucket DESC LIMIT 100;"
```

#### Manual Metrics Cleanup

```bash
# Delete metrics older than 60 days
sudo docker exec -it portopener-server-1 sqlite3 /data/portopener.db \
  "DELETE FROM metrics_rollup WHERE minute_bucket < strftime('%s', 'now', '-60 days');"
```

### Monitoring Commands

#### Check Service Health

```bash
# Check container status
sudo docker compose ps

# Check server health
curl http://localhost:8080/healthz

# Check Caddy status
sudo docker compose logs caddy | tail -50
```

#### Monitor Resource Usage

```bash
# Check Docker resource usage
sudo docker stats

# Check disk usage
df -h

# Check database size
sudo docker exec -it portopener-server-1 ls -lh /data/portopener.db
```

#### Monitor Active Tunnels

```bash
# List active tunnels
curl -H "Authorization: Bearer your-token" \
  https://admin.tunnel.example.com/api/tunnels

# List port reservations
curl -H "Authorization: Bearer your-token" \
  https://admin.tunnel.example.com/api/reservations/ports
```

### Alerting Recommendations

Consider setting up alerts for:

- **Service downtime**: Server or Caddy container stopped
- **Disk space**: Less than 20% free space
- **Failed TLS issuance**: Caddy logs showing certificate errors
- **High error rates**: HTTP 5xx responses in logs
- **Database size**: Database growing too quickly

---

## Troubleshooting

### Common Issues and Solutions

#### 1. TLS Ask Forbidden

**Symptom**: Caddy returns "forbidden" when requesting certificates for custom domains.

**Cause**: Domain not added to `custom_domains` table or status is not `enabled`.

**Solution**:

```bash
# Check if domain exists
curl -H "Authorization: Bearer your-token" \
  https://admin.tunnel.example.com/api/domains

# Add domain via API
curl -X POST \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{"domain":"myapp.example.com","tunnel_id":"tunnel-id","status":"enabled"}' \
  https://admin.tunnel.example.com/api/domains

# Verify ask endpoint
curl "http://localhost:8080/api/tls/ask?domain=myapp.example.com"
```

#### 2. Ports Not Open

**Symptom**: TCP/UDP tunnels not accessible from external clients.

**Cause**: Firewall blocking port range 20000-40000.

**Solution**:

```bash
# Check UFW status
sudo ufw status verbose

# Allow port range
sudo ufw allow 20000:40000/tcp
sudo ufw allow 20000:40000/udp

# Check cloud provider firewall
# (Log into your VPS provider's dashboard and verify rules)
```

#### 3. Database Permissions

**Symptom**: Server fails to start with "db open failed" error.

**Cause**: Incorrect permissions on database file or volume.

**Solution**:

```bash
# Check volume permissions
sudo docker volume inspect portopener_portopener_data

# Fix permissions
sudo docker run --rm -v portopener_portopener_data:/data alpine chown -R 1000:1000 /data

# Restart server
sudo docker compose restart server
```

#### 4. Caddy Logs Show Errors

**Symptom**: Certificates not issuing, Caddy logs show errors.

**Solution**:

```bash
# View Caddy logs
sudo docker compose logs caddy | tail -100

# Check Cloudflare API token
echo $CLOUDFLARE_API_TOKEN

# Verify DNS records
dig +short tunnel.example.com
dig +short admin.tunnel.example.com

# Test DNS resolution
nslookup tunnel.example.com
```

#### 5. Admin API Returns Unauthorized

**Symptom**: Admin API returns 401 Unauthorized even with correct token.

**Cause**: Token mismatch or IP not in allowlist.

**Solution**:

```bash
# Check your IP
curl https://ifconfig.me

# Verify allowlist includes your IP
echo $PORTOPENER_ADMIN_ALLOWLIST

# Test token
curl -H "Authorization: Bearer your-token" \
  http://localhost:8080/api/tunnels
```

#### 6. Database Locked

**Symptom**: Operations fail with "database is locked" error.

**Cause**: Multiple processes accessing SQLite simultaneously.

**Solution**:

```bash
# Restart server to release locks
sudo docker compose restart server

# Check for long-running transactions
sudo docker exec -it portopener-server-1 sqlite3 /data/portopener.db \
  "PRAGMA busy_timeout;"
```

#### 7. Container Keeps Restarting

**Symptom**: Server container exits and restarts repeatedly.

**Solution**:

```bash
# Check container logs
sudo docker compose logs server | tail -100

# Check if port is already in use
sudo netstat -tulpn | grep 8080

# Verify environment variables
sudo docker compose config
```

### Diagnostic Commands

```bash
# Full system check
sudo docker compose ps
sudo docker compose logs
sudo docker stats
df -h
free -h

# Database integrity check
sudo docker exec -it portopener-server-1 sqlite3 /data/portopener.db "PRAGMA integrity_check;"

# Check table sizes
sudo docker exec -it portopener-server-1 sqlite3 /data/portopener.db \
  "SELECT name, COUNT(*) FROM sqlite_master WHERE type='table' GROUP BY name;"
```

### Getting Help

If you encounter issues not covered here:

1. Check logs: `sudo docker compose logs -f`
2. Review this guide and [`docs/chat.md`](chat.md:1)
3. Check [`AGENTS.md`](../AGENTS.md:1) for architectural constraints
4. Open an issue on GitHub with:
   - VPS OS and version
   - Docker and Docker Compose versions
   - Full error messages
   - Relevant log output

---

## Verification Checklist

After deployment, verify the following:

- [ ] Docker and Docker Compose installed and running
- [ ] Firewall rules configured (80, 443, 20000-40000 TCP/UDP)
- [ ] Cloudflare DNS records created (tunnel, admin.tunnel)
- [ ] DNS records set to "DNS only" (gray cloud)
- [ ] Cloudflare API token created and configured
- [ ] Environment variables set in `.env` file
- [ ] Caddyfile updated with correct domain
- [ ] Docker Compose services started successfully
- [ ] Server health endpoint returns 200 OK
- [ ] Admin API accessible from allowed IP
- [ ] Admin UI loads in browser
- [ ] TLS certificates issued for admin.tunnel subdomain
- [ ] Database volume created and accessible
- [ ] Logs and metrics retention configured
- [ ] Backup strategy in place
- [ ] Token rotation procedure tested

---

## Next Steps

- Read [`docs/README.md`](README.md:1) for project overview
- Review [`docs/relay-protocol.md`](relay-protocol.md:1) for tunnel protocol details
- Check [`docs/techstack.md`](techstack.md:1) for technology stack information
- See [`deploy/README.md`](../deploy/README.md:1) for deployment-specific notes (if exists)
