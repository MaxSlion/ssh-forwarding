# Release v1.1.0

## What's New

### Features & Enhancements

*   **Static Port Mapping**:
    *   Added support for defining `static` ports in `server.yaml`.
    *   Clients can now bind to specific local ports (e.g., matching the server's port) instead of random ports.
    *   Useful for services requiring fixed ports like GitLab SSH.
    *   Config example:
        ```yaml
        - name: "GitLab SSH"
          target: "gitlab.com:22"
          static: true
          local_port: 2222
        ```
*   **New App Icon**:
    *   Updated the client application icon to a modern blue 3D network node design.

### Fixes

*   **Build**: resolved `context` package import errors during compilation.
*   **Icon Embedding**: Fixed icon embedding issue on Windows builds.

## Artifacts

*   `client-wails-v1.1-windows-amd64.zip`: Windows Client (GUI)
*   `server-v1.1-windows-amd64.zip`: Windows Server
*   `server-v1.1-linux-amd64.zip`: Linux Server
