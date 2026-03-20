# Release v1.2.0

> 发布日期：2026-03-20

## ✨ 新功能

- **静态端口映射**：`server.yaml` 支持 `static: true` + `local_port` 指定固定本地端口
- **自定义应用图标**：客户端嵌入蓝色网络节点图标 + Windows 清单 + 版本信息

## 🔧 代码优化

- **全局状态消除**：连接状态迁移至 `App` 结构体，消除竞态风险
- **连接泄漏修复**：`handleForwarding` 等待双向数据传输完成
- **设置项生效**：`connectionTimeout` 和 `localBindAddress` 正确传递到 SSH 连接
- **协议效率**：`Payload` 改为 `json.RawMessage`，消除双重序列化
- **安全 session 访问**：新增 `getSession()` 防止空指针 panic
- **服务端优雅退出**：采用 `signal.NotifyContext` 处理 SIGTERM/SIGINT
- **stdioConn.Close**：正确关闭 stdin/stdout 确保进程退出

## 🎨 UI 调整

- 移除设置页面主题切换 emoji
- 移除未使用的 SSH Config 输入框
- 版本号更新为 v1.2.0

## 📦 产物

- `client-wails-v1.2-windows-amd64.zip` — Windows 客户端 (GUI)
- `server-v1.2-windows-amd64.zip` — Windows 服务端
- `server-v1.2-linux-amd64.zip` — Linux 服务端
