#!/bin/bash
# Install required sing-box version
# Usage: ./install-sing-box.sh [install-dir]
# Default: /usr/local/bin
set -e

REQUIRED_VERSION="1.13.0-beta.8"
INSTALL_DIR="${1:-/usr/local/bin}"
SING_BOX="$INSTALL_DIR/sing-box"

# Check if already installed
if [[ -x "$SING_BOX" ]]; then
    CURRENT=$("$SING_BOX" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?' | head -1)
    if [[ "$CURRENT" == "$REQUIRED_VERSION" ]]; then
        echo "sing-box $REQUIRED_VERSION already installed at $SING_BOX"
        exit 0
    fi
    echo "Upgrading from $CURRENT to $REQUIRED_VERSION..."
fi

# Detect arch
ARCH=$(uname -m)
case "$ARCH" in
    arm64|aarch64) ARCH="arm64" ;;
    x86_64) ARCH="amd64" ;;
    *) echo "Unsupported: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
URL="https://github.com/SagerNet/sing-box/releases/download/v${REQUIRED_VERSION}/sing-box-${REQUIRED_VERSION}-${OS}-${ARCH}.tar.gz"

echo "Downloading sing-box v${REQUIRED_VERSION}..."
mkdir -p "$INSTALL_DIR"
curl -sL "$URL" | tar -xz -C /tmp
mv "/tmp/sing-box-${REQUIRED_VERSION}-${OS}-${ARCH}/sing-box" "$SING_BOX"
chmod +x "$SING_BOX"
rm -rf "/tmp/sing-box-${REQUIRED_VERSION}-${OS}-${ARCH}"

echo "Installed: $("$SING_BOX" version | head -1)"
