#!/usr/bin/env bash
# Run this script after SSH'ing into your Ubuntu/Debian VPS.
# Example:
#   ssh user@your-vps
#   curl -fsSL https://raw.githubusercontent.com/yourorg/PortOpener/main/deploy/vps-install.sh -o vps-install.sh
#   bash vps-install.sh
set -euo pipefail

log() {
  printf "\n==> %s\n" "$*"
}

wait_for_health() {
  local url="$1"
  local attempts="${2:-30}"
  local delay="${3:-2}"
  local i
  for i in $(seq 1 "$attempts"); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1
}

prompt_default() {
  local __var="$1"
  local __prompt="$2"
  local __default="$3"
  local __value
  read -r -p "$__prompt [$__default]: " __value
  if [ -z "$__value" ]; then
    __value="$__default"
  fi
  printf -v "$__var" '%s' "$__value"
}

prompt_required() {
  local __var="$1"
  local __prompt="$2"
  local __value
  while true; do
    read -r -p "$__prompt: " __value
    if [ -n "$__value" ]; then
      printf -v "$__var" '%s' "$__value"
      return
    fi
    echo "Value is required."
  done
}

prompt_secret() {
  local __var="$1"
  local __prompt="$2"
  local __value
  while true; do
    read -r -s -p "$__prompt: " __value
    echo
    if [ -n "$__value" ]; then
      printf -v "$__var" '%s' "$__value"
      return
    fi
    echo "Value is required."
  done
}

prompt_optional_secret() {
  local __var="$1"
  local __prompt="$2"
  local __value
  read -r -s -p "$__prompt (leave empty to skip): " __value
  echo
  printf -v "$__var" '%s' "$__value"
}

if [ -f /etc/os-release ]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  case "${ID:-}" in
    ubuntu|debian) ;;
    *)
      echo "Unsupported OS: ${ID:-unknown}. This script supports Ubuntu/Debian."
      exit 1
      ;;
  esac
else
  echo "/etc/os-release not found. Unsupported OS."
  exit 1
fi

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  if ! require_cmd sudo; then
    echo "sudo is required but not installed."
    exit 1
  fi
  SUDO="sudo"
fi

OWNER_USER="${SUDO_USER:-$USER}"

log "Installing prerequisites"
$SUDO apt-get update
$SUDO apt-get install -y ca-certificates curl gnupg lsb-release git

if ! require_cmd docker || ! docker compose version >/dev/null 2>&1; then
  log "Installing Docker Engine and Compose plugin"
  $SUDO install -m 0755 -d /etc/apt/keyrings
  curl -fsSL "https://download.docker.com/linux/${ID}/gpg" | $SUDO gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  $SUDO chmod a+r /etc/apt/keyrings/docker.gpg

  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${ID} $(lsb_release -cs) stable" | $SUDO tee /etc/apt/sources.list.d/docker.list >/dev/null
  $SUDO apt-get update
  $SUDO apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
  $SUDO systemctl enable --now docker
else
  log "Docker is already installed"
fi

log "Collecting deployment settings"
prompt_required REPO_URL "Repository git URL (e.g. https://github.com/yourorg/PortOpener.git)"
prompt_default INSTALL_DIR "Install directory" "/opt/portopener"

prompt_required BASE_DOMAIN "Base domain (example.com)"
prompt_required ACME_EMAIL "ACME email"
prompt_secret CLOUDFLARE_API_TOKEN "Cloudflare API token"
prompt_secret PORTOPENER_RELAY_TOKEN "Relay/admin token (openssl rand -base64 32)"
prompt_required PORTOPENER_ADMIN_ALLOWLIST "Admin allowlist CIDRs (comma-separated)"
prompt_optional_secret PORTOPENER_ADMIN_TOKEN "Admin token override"

if [ -d "$INSTALL_DIR" ] && [ -n "$(ls -A "$INSTALL_DIR" 2>/dev/null)" ]; then
  if [ -d "$INSTALL_DIR/.git" ]; then
    log "Existing repo detected. Pulling latest changes"
    (cd "$INSTALL_DIR" && git pull --ff-only)
  else
    echo "Install directory is not empty and not a git repo: $INSTALL_DIR"
    exit 1
  fi
else
  log "Cloning repository"
  $SUDO mkdir -p "$INSTALL_DIR"
  $SUDO chown "$OWNER_USER":"$OWNER_USER" "$INSTALL_DIR" || true
  git clone "$REPO_URL" "$INSTALL_DIR"
fi

DEPLOY_DIR="$INSTALL_DIR/deploy"
ENV_FILE="$DEPLOY_DIR/.env"

if [ ! -d "$DEPLOY_DIR" ]; then
  echo "Deploy directory not found: $DEPLOY_DIR"
  exit 1
fi

if [ -f "$ENV_FILE" ]; then
  BACKUP="$ENV_FILE.backup.$(date +%Y%m%d%H%M%S)"
  log "Backing up existing .env to $BACKUP"
  cp "$ENV_FILE" "$BACKUP"
fi

log "Writing .env configuration"
umask 077
cat > "$ENV_FILE" <<EOF
PORTOPENER_HTTP_ADDR=:8080
PORTOPENER_WEB_ROOT=/app/web
PORTOPENER_DB_PATH=/data/portopener.db
PORTOPENER_MIGRATIONS_DIR=/app/migrations
PORTOPENER_ADMIN_TOKEN=${PORTOPENER_ADMIN_TOKEN}
PORTOPENER_ADMIN_ALLOWLIST=${PORTOPENER_ADMIN_ALLOWLIST}
PORTOPENER_RELAY_TOKEN=${PORTOPENER_RELAY_TOKEN}
CLOUDFLARE_API_TOKEN=${CLOUDFLARE_API_TOKEN}
ACME_EMAIL=${ACME_EMAIL}
BASE_DOMAIN=${BASE_DOMAIN}
EOF

read -r -p "Configure UFW firewall rules for PortOpener? (y/N): " CONFIGURE_UFW
if [[ "${CONFIGURE_UFW}" =~ ^[Yy]$ ]]; then
  log "Configuring UFW"
  $SUDO apt-get install -y ufw
  $SUDO ufw --force enable
  $SUDO ufw allow 22/tcp
  $SUDO ufw allow 80/tcp
  $SUDO ufw allow 443/tcp
  $SUDO ufw allow 20000:40000/tcp
  $SUDO ufw allow 20000:40000/udp
  $SUDO ufw status verbose
fi

log "Building and starting containers"
(cd "$DEPLOY_DIR" && $SUDO docker compose build)
(cd "$DEPLOY_DIR" && $SUDO docker compose up -d)
(cd "$DEPLOY_DIR" && $SUDO docker compose ps)

log "Health check"
if wait_for_health "http://localhost:8080/healthz" 30 2; then
  echo "Server is healthy."
else
  echo "Health check failed. Check logs with: sudo docker compose -f ${DEPLOY_DIR}/docker-compose.yml logs"
  exit 1
fi

log "Deployment complete"
echo "Ensure DNS records are set for tunnel.${BASE_DOMAIN} and admin.tunnel.${BASE_DOMAIN} (DNS-only)."
