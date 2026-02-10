# ============================================================================
# build.ps1 - Cross-platform build script (run on Windows)
# ============================================================================
# Usage:
#   .\build.ps1              # Build all (server + windows client)
#   .\build.ps1 -Target server
#   .\build.ps1 -Target client
# ============================================================================

param(
    [ValidateSet("all", "server", "client")]
    [string]$Target = "all"
)

$ErrorActionPreference = "Stop"
$ROOT = $PSScriptRoot

$BIN_DIR = Join-Path $ROOT "dist"
New-Item -ItemType Directory -Force -Path $BIN_DIR | Out-Null

$FRONTEND_DIR = Join-Path $ROOT "cmd\client-wails\frontend"
$CLIENT_DIR   = Join-Path $ROOT "cmd\client-wails"
$SERVER_DIR   = Join-Path $ROOT "cmd\server"

# ── Helper ──────────────────────────────────────────────────────────────────
function Write-Step($msg) { Write-Host "`n>>> $msg" -ForegroundColor Cyan }

# ── Build Server (linux/amd64) ──────────────────────────────────────────────
function Build-Server {
    Write-Step "Building server for linux/amd64 (Ubuntu 22.04)..."

    $env:GOOS   = "linux"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"

    $outPath = Join-Path $BIN_DIR "server-linux-amd64"
    go build -trimpath -ldflags="-s -w" -o $outPath $SERVER_DIR

    # Reset env
    Remove-Item Env:\GOOS
    Remove-Item Env:\GOARCH
    Remove-Item Env:\CGO_ENABLED

    if ($LASTEXITCODE -ne 0) { throw "Server build failed" }
    Write-Host "  -> $outPath" -ForegroundColor Green
}

# ── Build Frontend ──────────────────────────────────────────────────────────
function Build-Frontend {
    Write-Step "Building frontend..."
    Push-Location $FRONTEND_DIR
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }
        Write-Host "  -> frontend/dist ready" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

# ── Build Client (Windows/amd64) ────────────────────────────────────────────
function Build-Client-Windows {
    Write-Step "Building client for windows/amd64..."

    $outPath = Join-Path $BIN_DIR "ssh-forwarder-windows-amd64.exe"

    Push-Location $CLIENT_DIR
    try {
        go build -trimpath -tags "desktop,production" `
            -ldflags="-s -w -H windowsgui" `
            -o $outPath .
        if ($LASTEXITCODE -ne 0) { throw "Windows client build failed" }
        Write-Host "  -> $outPath" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

# ── Main ────────────────────────────────────────────────────────────────────
Write-Host "========================================" -ForegroundColor Yellow
Write-Host " SSH Forwarder - Build Script" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Yellow

if ($Target -eq "all" -or $Target -eq "server") {
    Build-Server
}

if ($Target -eq "all" -or $Target -eq "client") {
    Build-Frontend
    Build-Client-Windows
}

Write-Host "`n========================================" -ForegroundColor Yellow
Write-Host " Build complete! Output in: dist/" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Yellow
Get-ChildItem $BIN_DIR | Format-Table Name, Length
