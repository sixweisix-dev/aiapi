#!/bin/bash
# ───────────────────────────────────────────────────────────────
# AI API Gateway — VPS Bootstrap Script
# Usage: curl -fsSL https://your-domain.com/setup.sh | bash
# Or:    bash scripts/setup.sh
#
# Tested on: Ubuntu 22.04 / 24.04
# ───────────────────────────────────────────────────────────────
set -euo pipefail

REPO_URL="${REPO_URL:-}"               # Git clone URL (required)
DEPLOY_DIR="${DEPLOY_DIR:-/opt/ai-api-gateway}"
DOMAIN="${DOMAIN:-}"                   # Your domain (required for HTTPS)
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"   # Will be generated if empty
JWT_SECRET="${JWT_SECRET:-}"           # Will be generated if empty

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# ── Pre-flight checks ─────────────────────────────────────────
if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root (or with sudo)"
fi

if [ -z "$DOMAIN" ]; then
    error "DOMAIN is required. Set it via: DOMAIN=api.yourdomain.com bash $0"
fi

# ── Step 1: System dependencies ───────────────────────────────
info "Installing system dependencies..."
apt-get update -qq
apt-get install -y -qq \
    ca-certificates curl gnupg lsb-release git \
    ufw

# ── Step 2: Docker ────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
    info "Installing Docker..."
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
        gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
        https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | \
        tee /etc/apt/sources.list.d/docker.list > /dev/null
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    systemctl enable docker
    systemctl start docker
    info "Docker installed"
else
    info "Docker already installed: $(docker --version)"
fi

# ── Step 3: Firewall ──────────────────────────────────────────
info "Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'SSH'
ufw allow 80/tcp comment 'HTTP'
ufw allow 443/tcp comment 'HTTPS'
ufw --force enable
info "Firewall: SSH, HTTP, HTTPS allowed"

# ── Step 4: Clone / update project ────────────────────────────
if [ -d "$DEPLOY_DIR" ]; then
    warn "Directory $DEPLOY_DIR already exists"
    read -rp "Overwrite? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        info "Skipping clone. Using existing directory."
    else
        rm -rf "$DEPLOY_DIR"
    fi
fi

if [ ! -d "$DEPLOY_DIR" ]; then
    if [ -n "$REPO_URL" ]; then
        info "Cloning repository..."
        git clone "$REPO_URL" "$DEPLOY_DIR"
    elif [ -d "/workspace/token-api" ]; then
        # Local copy (for development / manual deploy)
        info "Copying from local workspace..."
        cp -a /workspace/token-api "$DEPLOY_DIR"
    else
        error "No REPO_URL set and no local source found. Please set REPO_URL or copy files manually."
    fi
fi

cd "$DEPLOY_DIR"

# ── Step 5: Environment configuration ─────────────────────────
info "Configuring environment..."
if [ ! -f .env ]; then
    cp .env.example .env
else
    info ".env already exists, preserving"
fi

# Generate secrets if not provided
JWT_SECRET="${JWT_SECRET:-$(openssl rand -hex 32)}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-$(openssl rand -base64 12)}"

# Update critical values
sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${JWT_SECRET}|" .env
sed -i "s|^ADMIN_EMAIL=.*|ADMIN_EMAIL=${ADMIN_EMAIL}|" .env
sed -i "s|^ADMIN_PASSWORD=.*|ADMIN_PASSWORD=${ADMIN_PASSWORD}|" .env
sed -i "s|^ENVIRONMENT=.*|ENVIRONMENT=production|" .env
sed -i "s|^ENVIRONMENT=.*|ENVIRONMENT=production|" .env
sed -i "s|^CORS_ALLOW_ORIGINS=.*|CORS_ALLOW_ORIGINS=https://${DOMAIN}|" .env

# ── Step 6: Build and start ─────────────────────────────────────
info "Building frontend assets..."

if [ -d "frontend/admin" ] && [ -f "frontend/admin/package.json" ]; then
    cd frontend/admin && npm install --silent && npm run build && cd ../..
    info "Admin frontend built"
fi

if [ -d "frontend/user" ] && [ -f "frontend/user/package.json" ]; then
    cd frontend/user && npm install --silent && npm run build && cd ../..
    info "User frontend built"
fi

info "Starting services..."
docker compose up -d --build

# ── Step 7: Wait for health check ───────────────────────────────
info "Waiting for API to be healthy..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        info "API is healthy!"
        break
    fi
    sleep 2
done

# ── Step 8: Caddy HTTPS ─────────────────────────────────────────
info "Enabling HTTPS for ${DOMAIN}..."
CERT_EMAIL="${ADMIN_EMAIL}"
# Update Caddyfile: uncomment production block
sed -i "s/^# \(.*DOMAIN_PLACEHOLDER.*\)/\1/" "$DEPLOY_DIR/Caddyfile"
# Note: manual Caddyfile editing may be needed for your specific domain
warn "Caddyfile may need manual domain configuration. Edit ${DEPLOY_DIR}/Caddyfile"

docker compose restart caddy

# ── Step 9: Post-install info ──────────────────────────────────
echo ""
echo -e "${GREEN}==================================================${NC}"
echo -e "${GREEN}  AI API Gateway — Deployment Complete!${NC}"
echo -e "${GREEN}==================================================${NC}"
echo ""
echo "  Domain:        https://${DOMAIN}"
echo "  Admin Email:   ${ADMIN_EMAIL}"
echo "  Admin Pass:    ${ADMIN_PASSWORD}"
echo "  JWT Secret:    ${JWT_SECRET}"
echo ""
echo "  Useful commands:"
echo "    View logs:    docker compose logs -f"
echo "    Restart:      docker compose restart"
echo "    Backup:       make backup"
echo "    Shell:        docker compose exec postgres psql -U postgres ai_gateway"
echo ""
echo -e "${YELLOW}  IMPORTANT: Save the admin password above!${NC}"
echo -e "${YELLOW}  You can change it after first login.${NC}"
echo -e "${YELLOW}  Edit ${DEPLOY_DIR}/Caddyfile to set your domain for HTTPS.${NC}"
echo ""
echo -e "${GREEN}==================================================${NC}"

# ── Save credentials (root only) ───────────────────────────────
cat > /root/.ai_gateway_credentials << EOF
Admin URL:      https://${DOMAIN}/admin
Admin Email:    ${ADMIN_EMAIL}
Admin Password: ${ADMIN_PASSWORD}
JWT Secret:     ${JWT_SECRET}
Deploy Dir:     ${DEPLOY_DIR}
EOF
chmod 600 /root/.ai_gateway_credentials
info "Credentials saved to /root/.ai_gateway_credentials (root only)"
