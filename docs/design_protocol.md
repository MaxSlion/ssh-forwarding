# 高性能 SSH 端口转发系统设计方案

本文档旨在设计一套类似于 VS Code Remote 的高性能端口转发系统。该系统允许客户端在通过 SSH (每用户独立账号) 连接到仅开放 22 端口的服务器后，高效地访问服务器内部部署的各类 Web 服务 (如 GitLab, Ollama WebUI 等)。

## 1. 核心需求分析

-   **网络受限**: 仅允许 SSH (TCP 22) 通信。
-   **多用户隔离**: 每个用户使用独立的 SSH 系统账号登录。
-   **高性能**: 支持高吞吐量、低延迟，能够承载 WebUI 及 API 调用。
-   **易用性**: 客户端需具备图形界面 (GUI) 支持 Windows/macOS，提供登录、设置、端口列表展示及开关控制。
-   **动态发现**: 服务端通过配置文件控制允许访问的端口，并在连接建立时告知客户端。
-   **完备性**: 需完整实现握手、认证、转发及错误处理。

## 2. 架构设计

为了实现高性能和灵活性，采用 **Client-Side Agent (本地代理)** + **Server-Side Agent (服务端代理)** 的架构，通信隧道建立在标准 SSH 连接之上。

### 2.1 方案对比

| 方案 | 描述 | 优点 | 缺点 |
| :--- | :--- | :--- | :--- |
| **标准 SSH 隧道 (-L)** | 使用 `ssh -L` 本地端口转发 | 无需服务端部署额外软件，利用现有 SSHD。 | 连接数多时性能受限于 SSHD 实现；TCP over TCP 可能导致队头阻塞；配置管理分散。 |
| **自定义 Agent over SSH** | 客户端通过 SSH 执行服务端 Agent，通过 Stdin/Stdout 通信 | **高性能** (可使用 Yamux/QUIC 等现代多路复用协议)；绕过 SSHD 的 Channel 管理；功能可定制（如服务发现）。 | 需在服务端部署 Agent (可利用 SSH 自动上传或预装)。 |

### 2.2 推荐架构：基于 SSH Stio 的自定义多路复用

1.  **Transport Layer (传输层)**: 标准 SSH 连接 (Port 22)。
2.  **Session Layer (会话层)**: 客户端连接 SSH 后，通过 `Session.RequestExec("./server-agent")` 启动服务端代理。
3.  **Tunnel Layer (隧道层)**: 客户端与服务端 Agent 的 `Stdin/Stdout` 之间建立一条全双工流。
4.  **Application Layer (应用层)**:
    -   **Control Stream**: 用于交换配置、服务发现、心跳。
    -   **Data Streams**: 每个本地端口连接请求映射为一个虚拟 Stream。

此架构优势：
-   **绕过 SSHD 瓶颈**: SSHD 仅作为管道 (Pipe)，并不解析内部流量，减少了上下文切换。
-   **拥塞控制**: Yamux 等协议有更现代的流控算法。
-   **单连接**: 所有流量复用同一 TCP 连接，减少握手延迟。

## 3. 增强协议设计

为了支持服务发现和权限控制，在建立 Yamux 隧道后，需进行应用层握手。

### 3.1 握手流程 (Handshake)

1.  **Client -> Server**: 发送 `HELLO` 报文 (可选包含客户端版本)。
2.  **Server -> Client**: 返回 `CONFIG` 报文。
    ```json
    {
      "version": "1.0",
      "allowed_ports": [
        {"name": "GitLab", "target": "localhost:8080", "description": "Code Repository"},
        {"name": "Ollama", "target": "localhost:11434", "description": "AI Model Serving"}
      ]
    }
    ```
3.  **Client**: 渲染 GUI 列表。

### 3.2 转发请求

当用户在 GUI 点击 "开启" 时：
1.  Client 发起 Yamux Stream。
2.  Stream 头发送目标请求: `CONNECT localhost:8080`。
3.  Server 校验 `localhost:8080` 是否在允许列表中。
    -   是: 建立连接，开始透传。
    -   否: 关闭 Stream。

## 4. 客/服务端详细设计

### 4.1 客户端 (GUI)

采用 **Fyne** 或 **Wails** (推荐 Fyne 以获得原生简单的 Go 绑定) 开发跨平台 GUI。

#### 4.1.1 界面交互
1.  **登录页 (Login)**:
    -   输入: 用户名 (User), 密码 (Password)。
    -   设置 (齿轮):配置服务端 IP:Port (默认空，需填写)。
    -   动作: 点击 "Login" -> 建立 SSH 连接 -> 启动 Agent -> 获取配置 -> 跳转主页。
2.  **主页 (Dashboard)**:
    -   显示服务端返回的 `allowed_ports` 列表。
    -   每行: 服务名 | 远程地址 | 本地监听地址 (输入框, 默认 localhost:随机) | 开关 (Toggle)。
    -   动作:
        -   Toggle On: 启动本地 Listener，建立映射。
        -   Toggle Off: 关闭 Listener。

#### 4.1.2 核心模块
-   **SSH Manager**: 管理 SSH Client 会话，支持 Password Auth。
-   **Tunnel Manager**: 维护 Yamux Session。
-   **Port Forwarder**: 动态启动/停止 `net.Listener`。

### 4.2 服务端 (Server Agent)

服务端需增加配置文件和权限校验。

#### 4.2.1 配置文件 (`server.yaml`)
```yaml
# server.yaml
allowed_ports:
  - name: "GitLab Web"
    target: "127.0.0.1:80"
  - name: "Ollama API"
    target: "127.0.0.1:11434"
```

#### 4.2.2 启动逻辑
1.  加载 `server.yaml`。
2.  启动多路复用监听 (Stdio)。
3.  等待 Control Stream (Stream ID 1) 或在首个 Stream 发送配置? 
    -   *更优方案*: 建立连接后，服务端主动在 Stdio 的首个数据包发送 Config JSON，或者约定 Stream 0 为控制流。
    -   *简化方案*: 客户端连接后，发送个 "GetConfig" 请求，服务端回包。

### 4.3 常见问题答疑
-   **Q: 服务端是否监听 22 端口?**
    -   **A**: 不。服务端 SSHD (系统服务) 监听 22 端口。我们的 `server-agent` 是一个普通的可执行文件。当客户端通过 SSH 登录后，会自动执行 `./server-agent`。它复用 SSH 的加密通道，不监听任何额外的服务器端口。

## 5. 性能与安全
(保留原有优化策略)


### 3.1 客户端 (Client)

客户端为一个运行在用户本地的守护进程或 CLI 工具。

-   **配置管理**: 读取 `config.yaml`，定义转发规则。
    ```yaml
    # config.yaml 示例
    server: 192.168.1.100
    user: myuser
    auth: key_file # or password
    forwards:
      - local: 8080
        remote: localhost:8080 # GitLab
      - local: 11434
        remote: localhost:11434 # Ollama
    ```
-   **连接管理**:
    1.  使用 Go `crypto/ssh` 库建立 SSH 连接。
    2.  请求执行服务端 Agent (`/usr/local/bin/forward-server` 或上传临时二进制)。
    3.  获取远程命令的 `StdinPipe` 和 `StdoutPipe`。
    4.  初始化多路复用器 (Multiplexer Client) 绑定到这两个管道。
-   **端口监听**:
    -   根据配置在本地启动 TCP Listener。
    -   当收到连接时，Accept 连接 -> 在 Multiplexer 上 OpenStream -> 将本地 Conn 与 Stream 进行 `io.Copy`。

### 3.2 服务端 (Server Agent)

服务端 Agent 是一个轻量级二进制文件，无需 root 权限 (绑定非特权端口)。

-   **启动模式**:
    -   被 SSH 调用启动：`forward-server --stdio`。此时它直接使用 `os.Stdin` 和 `os.Stdout` 进行通信。
-   **多路复用逻辑**:
    -   初始化多路复用器 (Multiplexer Server) 绑定到 Stdin/Stdout。
    -   等待 Accept Stream。
-   **请求处理**:
    -   解析 Stream 头部协议 (自定义协议头，包含目标 IP:Port)。
    -   发起 `net.Dial("tcp", target)` 连接目标服务 (如 GitLab 端口)。
    -   将 Stream 与目标服务 Conn 进行双向 Copy。

## 4. 性能优化策略

### 4.1 内存与缓冲 (Zero-Copy & Buffering)
-   **Go `io.Copy` 优化**: Go 的 `io.Copy` 在 Linux 上会自动利用 `splice` 系统调用及 `sendfile` (如果适用)，减少用户态与内核态的数据拷贝。
-   **Buffer Pool**: 使用 `sync.Pool` 复用字节缓冲区 (例如 32KB 或 64KB chunks)，减少 GC 压力。

### 4.2 多路复用协议调优 (Yamux)
-   **Window Size**: 增大接收窗口大小 (默认 256KB 可调至 1MB+)，以充分利用内网高带宽。
-   **KeepAlive**: 设置应用层心跳，避免防火墙切断空闲连接。

### 4.3 连接复用
-   **Single SSH Connection**: 默认复用一条 SSH 链接。如果单连接带宽受限 (例如 TCP Window 限制)，可设计为自动建立多条 SSH 连接进行负载均衡 (Connection Pool)，例如每 100 个并发连接开启一个新的 SSH 通道。

## 5. 安全性考虑

-   **SSH 原生安全**: 利用 SSH 本身的加密和认证 (RSA/Ed25519 Keys)，不仅无需处理 TLS 证书，且天然通过了防火墙。
-   **权限控制**: 服务端 Agent 以登录用户的权限运行。用户只能访问其 SSH 账号有权限访问的网络资源。
-   **绑定地址**: 客户端默认监听 `127.0.0.1`，防止本地端口暴露给局域网其他设备 (可配置)。

## 6. 技术选型推荐

-   **语言**: Go (Golang)
    -   **理由**: 原生并发 (Goroutines) 处理成千上万连接非常高效；拥有成熟的 SSH 库 (`golang.org/x/crypto/ssh`) 和多路复用库 (`hashicorp/yamux`)；跨平台编译方便 (Windows/Linux/macOS)。
-   **关键库**:
    -   SSH 客户端: `golang.org/x/crypto/ssh`
    -   多路复用: `github.com/hashicorp/yamux`
    -   CLI 框架: `github.com/spf13/cobra`

## 7. 实施路线图

1.  **Prototype**: 实现完整的 Go 程序，包含 Client 和 Server，硬编码通过 Stdio 传输数据。
2.  **Protocol**: 定义完整的握手协议。
3.  **Integration**: 集成 SSH 库，实现 Client 自动拨号并启动 Server。
4.  **Config & UI**: 增加配置文件支持，开发 GUI 界面。

---

**总结**: 采用 "SSH 隧道 + 自定义多路复用 Agent" 的方案，既满足了单一 22 端口的限制，又能通过应用层优化突破标准 SSH 转发的性能瓶颈，是实现类似 VS Code Remote 体验的最佳实践。

**调试版本编译**
cd cmd/client-wails/frontend
npm run build
cd ..
go build -tags desktop,production -o build/bin/client-wails.exe .

**生产版本编译**
cd cmd/client-wails/frontend
npm run build
cd ..
go build -ldflags "-H windowsgui" -tags desktop,production -o build/bin/client-wails.exe .