#!/usr/bin/env bash
# ============================================================================
# build-macos.sh - Build client for macOS (must run on macOS)
# ============================================================================
# Usage:
#   ./build-macos.sh              # Build client-wails.app for current macOS arch
#   ./build-macos.sh --universal  # Build universal client-wails.app (amd64 + arm64)
# ============================================================================

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$ROOT/dist"
mkdir -p "$BIN_DIR"

FRONTEND_DIR="$ROOT/cmd/client-wails/frontend"
CLIENT_DIR="$ROOT/cmd/client-wails"

if command -v wails >/dev/null 2>&1; then
    WAILS_BIN="wails"
elif [[ -x "$HOME/go/bin/wails" ]]; then
    WAILS_BIN="$HOME/go/bin/wails"
else
    echo "ERROR: wails CLI not found. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

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

# ── Build Client App (macOS release) ───────────────────────────────────────
cd "$CLIENT_DIR"

BUILD_PLATFORM="darwin/$(go env GOARCH)"
if $UNIVERSAL; then
    BUILD_PLATFORM="darwin/universal"
fi

echo ""
echo ">>> Building release app bundle ($BUILD_PLATFORM)..."
CGO_LDFLAGS="${CGO_LDFLAGS:-} -framework UniformTypeIdentifiers" \
    "$WAILS_BIN" build -clean -tags "desktop,production" -platform "$BUILD_PLATFORM"

APP_SRC="$CLIENT_DIR/build/bin/client-wails.app"
APP_DST="$BIN_DIR/client-wails.app"

if [[ ! -d "$APP_SRC" ]]; then
    echo "ERROR: Expected app bundle not found at $APP_SRC"
    exit 1
fi

rm -rf "$APP_DST"
cp -R "$APP_SRC" "$APP_DST"

echo "  -> $APP_DST"

echo ""
echo "========================================"
echo " macOS release app build complete!"
echo "========================================"
ls -la "$BIN_DIR"/client-wails.app
