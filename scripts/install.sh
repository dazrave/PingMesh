#!/bin/bash
set -euo pipefail

BINARY_NAME="pingmesh"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/pingmesh"
SERVICE_FILE="/etc/systemd/system/pingmesh.service"
GITHUB_REPO="pingmesh/pingmesh"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()   { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[-]${NC} $*" >&2; exit 1; }

detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) error "Unsupported architecture: $arch" ;;
    esac
}

detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux) echo "linux" ;;
        *) error "Unsupported OS: $os (only Linux is supported)" ;;
    esac
}

install() {
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)

    log "Detected platform: ${os}/${arch}"

    if [ "$(id -u)" -ne 0 ]; then
        error "This script must be run as root (use sudo)"
    fi

    # Create user
    if ! id -u "$BINARY_NAME" &>/dev/null; then
        log "Creating ${BINARY_NAME} user..."
        useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$BINARY_NAME"
    fi

    # Create data directory
    log "Creating data directory..."
    mkdir -p "$DATA_DIR"
    chown "$BINARY_NAME:$BINARY_NAME" "$DATA_DIR"
    chmod 750 "$DATA_DIR"

    # Download binary
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep tag_name | cut -d'"' -f4)
    if [ -z "$version" ]; then
        error "Failed to determine latest version"
    fi

    log "Downloading ${BINARY_NAME} ${version}..."
    local url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${BINARY_NAME}-${os}-${arch}"
    curl -fsSL "$url" -o "${INSTALL_DIR}/${BINARY_NAME}"
    chmod 755 "${INSTALL_DIR}/${BINARY_NAME}"

    # Install systemd service
    log "Installing systemd service..."
    curl -fsSL "https://raw.githubusercontent.com/${GITHUB_REPO}/${version}/configs/pingmesh.service" -o "$SERVICE_FILE"
    systemctl daemon-reload
    systemctl enable "$BINARY_NAME"

    log "Installation complete!"
    log "Run '${BINARY_NAME} init' to initialize as coordinator, or '${BINARY_NAME} join <token>' to join a cluster."
}

uninstall() {
    if [ "$(id -u)" -ne 0 ]; then
        error "This script must be run as root (use sudo)"
    fi

    warn "Uninstalling ${BINARY_NAME}..."

    systemctl stop "$BINARY_NAME" 2>/dev/null || true
    systemctl disable "$BINARY_NAME" 2>/dev/null || true
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"

    warn "Data directory ${DATA_DIR} has NOT been removed. Remove it manually if desired."
    log "Uninstall complete."
}

upgrade() {
    if [ "$(id -u)" -ne 0 ]; then
        error "This script must be run as root (use sudo)"
    fi

    log "Stopping service..."
    systemctl stop "$BINARY_NAME" 2>/dev/null || true

    install

    log "Starting service..."
    systemctl start "$BINARY_NAME"
    log "Upgrade complete!"
}

usage() {
    echo "Usage: $0 [--install|--uninstall|--upgrade]"
    echo ""
    echo "Options:"
    echo "  --install     Install PingMesh (default)"
    echo "  --uninstall   Remove PingMesh"
    echo "  --upgrade     Upgrade PingMesh to latest version"
    echo "  --no-start    Install without starting the service"
}

case "${1:---install}" in
    --install)   install ;;
    --uninstall) uninstall ;;
    --upgrade)   upgrade ;;
    --help|-h)   usage ;;
    *)           usage; exit 1 ;;
esac
