# Release v1.0

## Features
- **Modern GUI Client**: Windows application built with Wails (React + Go).
- **SSH Tunneling**: Secure port forwarding over SSH.
- **Dynamic Forwarding**: Start/stop forwarded ports on demand.
- **Traffic Monitoring**: Real-time bandwidth visualization.
- **Configuration**: YAML-based server configuration for allowed ports.

## Artifacts
- **Client**: `client-wails-v1.0-windows-amd64.zip` (Windows 10/11)
- **Server**: `server-v1.0-linux-amd64.zip` (Linux AMD64)

## Installation

### Server (Linux)
1.  Upload `server` binary to your remote machine.
2.  Create `server.yaml` configuration file.
3.  Run: `./server` (or set up as a systemd service).

### Client (Windows)
1.  Unzip `client-wails-v1.0-windows-amd64.zip`.
2.  Run `client-wails.exe`.
3.  Enter SSH credentials and connect.
