#!/usr/bin/env bash
# ============================================================================
# build-macos.sh - Build client for macOS (must run on macOS)
# ============================================================================
# Usage:
#   ./build-macos.sh              # Build client for macOS
#   ./build-macos.sh --universal  # Build universal binary (amd64 + arm64)
# ============================================================================

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$ROOT/dist"
mkdir -p "$BIN_DIR"

FRONTEND_DIR="$ROOT/cmd/client-wails/frontend"
CLIENT_DIR="$ROOT/cmd/client-wails"

UNIVERSAL=false
if [[ "${1:-}" == "--universal" ]]; then
    UNIVERSAL=true
fi

echo "========================================"
echo " SSH Forwarder - macOS Build Script"
echo "========================================"

# ── Build Frontend ──────────────────────────────────────────────────────────
echo ""
echo ">>> Building frontend..."
cd "$FRONTEND_DIR"
npm run build
echo "  -> frontend/dist ready"

# ── Build Client (macOS) ────────────────────────────────────────────────────
cd "$CLIENT_DIR"

if $UNIVERSAL; then
    echo ""
    echo ">>> Building universal macOS binary (amd64 + arm64)..."

    # Build amd64
    echo "  -> Building amd64..."
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 \
        go build -trimpath -tags "desktop,production" \
        -ldflags="-s -w" \
        -o "$BIN_DIR/ssh-forwarder-darwin-amd64" .

    # Build arm64
    echo "  -> Building arm64..."
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 \
        go build -trimpath -tags "desktop,production" \
        -ldflags="-s -w" \
        -o "$BIN_DIR/ssh-forwarder-darwin-arm64" .

    # Create universal binary
    echo "  -> Creating universal binary..."
    lipo -create -output "$BIN_DIR/ssh-forwarder-darwin-universal" \
        "$BIN_DIR/ssh-forwarder-darwin-amd64" \
        "$BIN_DIR/ssh-forwarder-darwin-arm64"

    echo "  -> $BIN_DIR/ssh-forwarder-darwin-universal"
else
    # Detect current arch
    ARCH=$(uname -m)
    if [[ "$ARCH" == "x86_64" ]]; then
        GOARCH="amd64"
    else
        GOARCH="arm64"
    fi

    echo ""
    echo ">>> Building client for darwin/$GOARCH..."
    GOOS=darwin GOARCH=$GOARCH CGO_ENABLED=1 \
        go build -trimpath -tags "desktop,production" \
        -ldflags="-s -w" \
        -o "$BIN_DIR/ssh-forwarder-darwin-$GOARCH" .

    echo "  -> $BIN_DIR/ssh-forwarder-darwin-$GOARCH"
fi

echo ""
echo "========================================"
echo " macOS build complete! Output in: dist/"
echo "========================================"
ls -lh "$BIN_DIR"/ssh-forwarder-darwin-*
