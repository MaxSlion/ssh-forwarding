# Release v1.2.0

## New Features

- **Static port mapping**: server.yaml supports static: true + local_port to specify a fixed local port
- **Custom application icon**: Client embeds a blue network node icon + Windows manifest + version information

## Code Optimizations

- **Eliminated global state**: Connection state migrated to the App struct, eliminating race condition risks
- **Connection leak fix**: handleForwarding now waits for bidirectional data transfer completion
- **Settings take effect**: connectionTimeout and localBindAddress are correctly passed to the SSH connection
- **Protocol efficiency**: Changed Payload to json.RawMessage to eliminate double serialization
- **Safe session access**: Added getSession() to prevent null pointer panics
- **Graceful server shutdown**: Use signal.NotifyContext to handle SIGTERM/SIGINT
- **stdioConn.Close**: Properly close stdin/stdout to ensure process exit

## UI Adjustments

- **Removed theme switching emoji from settings page**
- **Removed unused SSH Config input field**
- **Version number updated to v1.2.0**

## Artifacts

- `client-wails-v1.2-windows-amd64.zip` — Windows client (GUI)
- `client-wails-v1.2-macos-arm64.zip` — macOS client (GUI)
- `server-v1.2-linux-amd64.zip` — Linux server