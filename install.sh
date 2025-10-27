#!/bin/bash
# Graphium Installation Script

set -e

VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="graphium"

echo "🧬 Installing Graphium..."

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Download URL
if [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/evalgo/graphium/releases/latest/download/graphium-${OS}-${ARCH}"
else
    DOWNLOAD_URL="https://github.com/evalgo/graphium/releases/download/${VERSION}/graphium-${VERSION}-${OS}-${ARCH}"
fi

if [ "$OS" = "windows" ]; then
    BINARY_NAME="${BINARY_NAME}.exe"
    DOWNLOAD_URL="${DOWNLOAD_URL}.exe"
fi

echo "Downloading from: $DOWNLOAD_URL"

# Download
TMP_FILE="/tmp/${BINARY_NAME}"
curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"

# Make executable
chmod +x "$TMP_FILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
else
    echo "Installing to $INSTALL_DIR requires sudo..."
    sudo mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
fi

echo "✓ Graphium installed to $INSTALL_DIR/$BINARY_NAME"
echo ""
echo "Run 'graphium version' to verify installation"
