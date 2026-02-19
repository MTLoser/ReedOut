#!/usr/bin/env bash
#
# ReedOut — Proxmox LXC Install Script
# Usage: bash -c "$(curl -fsSL https://raw.githubusercontent.com/MTLoser/ReedOut/main/scripts/install-proxmox.sh)"
#
set -euo pipefail

# --- Configuration (override via environment) ---
CTID="${CTID:-}"
HOSTNAME="${HOSTNAME:-reedout}"
MEMORY="${MEMORY:-2048}"
SWAP="${SWAP:-512}"
CORES="${CORES:-2}"
DISK_SIZE="${DISK_SIZE:-20}"
STORAGE="${STORAGE:-local-lvm}"
TEMPLATE_STORAGE="${TEMPLATE_STORAGE:-local}"
BRIDGE="${BRIDGE:-vmbr0}"
PASSWORD="${PASSWORD:-reedout}"
REPO="MTLoser/ReedOut"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# --- Preflight checks ---
command -v pveversion >/dev/null 2>&1 || error "This script must be run on a Proxmox VE host."
[[ "$(id -u)" -eq 0 ]] || error "This script must be run as root."

echo ""
echo -e "${GREEN}╔══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║      ReedOut Proxmox Installer       ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════╝${NC}"
echo ""

# --- Get next available CTID ---
if [[ -z "$CTID" ]]; then
    CTID=$(pvesh get /cluster/nextid)
fi
info "Using CT ID: $CTID"

# --- Find or download Debian template ---
TEMPLATE=""
for t in $(pveam list "$TEMPLATE_STORAGE" 2>/dev/null | awk '/debian-1[23].*standard/ {print $1}' | sort -r); do
    TEMPLATE="$t"
    break
done

if [[ -z "$TEMPLATE" ]]; then
    info "Downloading Debian 12 template..."
    pveam update >/dev/null 2>&1
    TEMPLATE_FILE=$(pveam available --section system | awk '/debian-12.*standard/ {print $2}' | head -1)
    [[ -n "$TEMPLATE_FILE" ]] || error "Could not find Debian template to download."
    pveam download "$TEMPLATE_STORAGE" "$TEMPLATE_FILE"
    TEMPLATE="${TEMPLATE_STORAGE}:vztmpl/${TEMPLATE_FILE}"
fi
ok "Using template: $TEMPLATE"

# --- Create LXC container ---
info "Creating LXC container $CTID ($HOSTNAME)..."
pct create "$CTID" "$TEMPLATE" \
    --hostname "$HOSTNAME" \
    --memory "$MEMORY" \
    --swap "$SWAP" \
    --cores "$CORES" \
    --rootfs "${STORAGE}:${DISK_SIZE}" \
    --net0 "name=eth0,bridge=${BRIDGE},ip=dhcp" \
    --features nesting=1,keyctl=1 \
    --unprivileged 0 \
    --password "$PASSWORD" \
    --start 1 \
    --ostype debian
ok "Container $CTID created and started."

# Wait for network
info "Waiting for network..."
for i in $(seq 1 30); do
    IP=$(pct exec "$CTID" -- bash -c "ip -4 addr show eth0 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}'" 2>/dev/null || true)
    [[ -n "$IP" ]] && break
    sleep 1
done
[[ -n "$IP" ]] || error "Container did not get an IP address after 30s."
ok "Container IP: $IP"

# --- Install Docker ---
info "Installing Docker (this may take a minute)..."
pct exec "$CTID" -- bash -c "
    apt-get update -qq >/dev/null 2>&1
    apt-get install -y -qq curl ca-certificates >/dev/null 2>&1
    curl -fsSL https://get.docker.com | sh >/dev/null 2>&1
" || error "Failed to install Docker."
ok "Docker installed."

# --- Download and install ReedOut ---
info "Downloading latest ReedOut release..."
RELEASE_URL="https://api.github.com/repos/${REPO}/releases/latest"
DOWNLOAD_URL=$(pct exec "$CTID" -- bash -c "
    curl -fsSL '$RELEASE_URL' 2>/dev/null | grep -o 'https://[^\"]*reedout-linux-amd64.tar.gz' | head -1
" 2>/dev/null || true)

if [[ -z "$DOWNLOAD_URL" ]]; then
    warn "No release found. Building from source..."
    pct exec "$CTID" -- bash -c "
        # Install Go
        curl -fsSL https://go.dev/dl/go1.23.7.linux-amd64.tar.gz | tar -C /usr/local -xzf -
        export PATH=\$PATH:/usr/local/go/bin

        # Install Node
        curl -fsSL https://deb.nodesource.com/setup_22.x | bash - >/dev/null 2>&1
        apt-get install -y -qq nodejs git >/dev/null 2>&1

        # Clone and build
        git clone https://github.com/${REPO}.git /opt/reedout
        cd /opt/reedout
        go build -o reedout ./cmd/reedout
        cd web && npm install --silent && npx vite build
    " || error "Failed to build from source."
else
    pct exec "$CTID" -- bash -c "
        mkdir -p /opt/reedout
        cd /opt/reedout
        curl -fsSL '$DOWNLOAD_URL' | tar xzf -
    " || error "Failed to download release."
fi
ok "ReedOut installed."

# --- Create systemd service ---
info "Setting up systemd service..."
pct exec "$CTID" -- bash -c "
    mkdir -p /opt/reedout/data

    cat > /etc/systemd/system/reedout.service << 'SVCEOF'
[Unit]
Description=ReedOut Game Server Manager
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
WorkingDirectory=/opt/reedout
ExecStart=/opt/reedout/reedout
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
SVCEOF

    systemctl daemon-reload
    systemctl enable reedout
    systemctl start reedout
"
ok "Service started."

# --- Done! ---
sleep 2
echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║         ReedOut installed successfully!       ║${NC}"
echo -e "${GREEN}╠══════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║${NC}  URL:      ${CYAN}http://${IP}:8080${NC}                  ${GREEN}║${NC}"
echo -e "${GREEN}║${NC}  Login:    ${CYAN}admin / admin${NC}                      ${GREEN}║${NC}"
echo -e "${GREEN}║${NC}  CT ID:    ${CYAN}${CTID}${NC}                                    ${GREEN}║${NC}"
echo -e "${GREEN}║${NC}                                              ${GREEN}║${NC}"
echo -e "${GREEN}║${NC}  ${YELLOW}Change the default password after login!${NC}    ${GREEN}║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════╝${NC}"
echo ""
