#!/bin/bash
# Importinvoices installer — Unix
# Usage: curl -fsSL https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.sh | bash

set -e

REPO="paleicikas/importinvoices"
BINARY="importinvoices"
INSTALL_DIR="${IMPORTINVOICES_INSTALL_DIR:-$HOME/.local/bin}"

echo "==> Importinvoices installer"

VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "No GitHub release found yet. Build from source instead:"
    echo "  cd server; go install ./cmd/importinvoices"
    exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
if [ "$ARCH" == "x86_64" ]; then ARCH="x86_64"; fi
if [ "$ARCH" == "aarch64" ] || [ "$ARCH" == "arm64" ]; then ARCH="arm64"; fi

PLATFORM="${OS}_${ARCH}"
URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY}_${PLATFORM}.tar.gz"
TMPDIR=$(mktemp -d)

echo "==> Downloading $BINARY $VERSION ($PLATFORM)"
curl -fsSL "$URL" | tar -xz -C "$TMPDIR"

mkdir -p "$INSTALL_DIR"
mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo "==> Installed to $INSTALL_DIR/$BINARY"
echo "==> Run: importinvoices serve"
