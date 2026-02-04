Enter these values when prompted
Repository git URL: https://github.com/AidyyJ/PortOpener.git
Install directory: accept default (/opt/portopener) from prompt_default INSTALL_DIR
Base domain: 100tunnels.xyz from prompt_required BASE_DOMAIN
ACME email: your email (Let’s Encrypt notifications) from prompt_required ACME_EMAIL
Cloudflare API token: your token (kept secret) from prompt_secret CLOUDFLARE_API_TOKEN
Relay/admin token: paste a strong token (or generate) from prompt_secret PORTOPENER_RELAY_TOKEN
generator (run locally on VPS in another shell): openssl rand -base64 32
Admin allowlist CIDRs: 0.0.0.0/0 (temporary open) from prompt_required PORTOPENER_ADMIN_ALLOWLIST
Admin token override: leave empty (unless you want a separate token) from prompt_optional_secret PORTOPENER_ADMIN_TOKEN
These values get written into /opt/portopener/deploy/.env via the heredoc at cat > "$ENV_FILE". Caddy uses them in deploy/Caddyfile, including the Cloudflare DNS-01 issuer at dns cloudflare {env.CLOUDFLARE_API_TOKEN}.

4) Verify on the VPS
The script already checks health at wait_for_health "http://localhost:8080/healthz".

You can also run:

cd /opt/portopener/deploy
sudo docker compose ps
sudo docker compose logs -f --tail=200

5) Important security note (because you chose 0.0.0.0/0)
With PORTOPENER_ADMIN_ALLOWLIST=0.0.0.0/0, the admin surface is internet-reachable and protected only by your token (see admin restrictions in docs/vps-deployment.md).

As soon as you’re done verifying, lock it down:

Edit /opt/portopener/deploy/.env and set PORTOPENER_ADMIN_ALLOWLIST=<your-public-ip>/32
Restart server:
cd /opt/portopener/deploy
sudo docker compose restart server