#!/bin/bash
# Importinvoices installer — Unix
# Usage: curl -fsSL https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.sh | bash

set -e

REPO="paleicikas/importinvoices"
BINARY="importinvoices"
INSTALL_DIR="${IMPORTINVOICES_INSTALL_DIR:-$HOME/.local/bin}"

echo "==> Importinvoices installer"

# Find latest release with assets
RELEASES=$(curl -s "https://api.github.com/repos/$REPO/releases")
VERSION=""
ASSET_URL=""

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
if [ "$ARCH" == "x86_64" ]; then ARCH="x86_64"; fi
if [ "$ARCH" == "aarch64" ] || [ "$ARCH" == "arm64" ]; then ARCH="arm64"; fi

PLATFORM="${OS}_${ARCH}"
EXTENSION="tar.gz"
if [ "$OS" == "windows" ]; then EXTENSION="zip"; fi
ASSET_NAME="${BINARY}_${PLATFORM}.${EXTENSION}"

# Parse releases to find one with the matching asset
# This is a bit tricky with just curl and grep/sed, but we can try
VERSION=$(echo "$RELEASES" | grep -oP '"tag_name":\s*"\K[^"]+' | head -n 1)

if [ -z "$VERSION" ]; then
    echo "No GitHub release found yet. Build from source instead:"
    echo "  cd server; go install ./cmd/importinvoices"
    exit 1
fi

# Try to find the specific asset URL
ASSET_URL=$(echo "$RELEASES" | grep -oP '"browser_download_url":\s*"\K[^"]+' | grep "$ASSET_NAME" | head -n 1)

if [ -z "$ASSET_URL" ]; then
    echo "No binary found for $PLATFORM in latest releases. Build from source instead:"
    echo "  cd server; go install ./cmd/importinvoices"
    exit 1
fi

TMPDIR=$(mktemp -d)

echo "==> Downloading $BINARY $VERSION ($PLATFORM)"
curl -fsSL "$ASSET_URL" | tar -xz -C "$TMPDIR"

mkdir -p "$INSTALL_DIR"
mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo "==> Installed to $INSTALL_DIR/$BINARY"
echo "==> Run: importinvoices serve"
