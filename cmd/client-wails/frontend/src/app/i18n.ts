// i18n translations for SSH Forwarder
export type Language = "zh" | "en";

export interface Translations {
    // App
    appTitle: string;

    // Window controls
    minimize: string;
    maximize: string;
    restore: string;
    close: string;

    // Sidebar
    savedConnections: string;
    noSavedConnections: string;

    // Connection form
    sshConfig: string;
    fillServerInfo: string;
    hostAddress: string;
    username: string;
    password: string;
    authMethod: string;
    passwordAuth: string;
    sshConfigOptional: string;
    sshConfigHint: string;
    saveThisConnection: string;
    connectionName: string;
    testConnection: string;
    testing: string;
    connect: string;
    connecting: string;
    sshProtocol: string;

    // Connected view
    connectedTo: string;
    user: string;
    forwardablePorts: string;
    startForward: string;
    stopForward: string;
    noForwardPorts: string;
    disconnect: string;

    // Saved connections menu
    rename: string;
    delete: string;
    deleteConfirm: string;

    // Status bar
    ready: string;
    connected: string;
    disconnected: string;
    upload: string;
    download: string;
    forwardingInfo: string;

    // Test connection results
    testSuccess: string;
    testFailed: string;
    connRefused: string;
    connTimeout: string;
    noRoute: string;
    authFailed: string;
    authNotSupported: string;
    connFailed: string;
    testingConnection: string;

    // Settings
    settings: string;
    connectionSettings: string;
    agentPath: string;
    agentPathHint: string;
    connectionTimeout: string;
    connectionTimeoutUnit: string;
    localBindAddress: string;
    localOnly: string;
    lanShare: string;
    autoReconnect: string;
    autoReconnectHint: string;
    appearance: string;
    theme: string;
    themeLight: string;
    themeDark: string;
    language: string;
    resetDefaults: string;
    cancel: string;
    save: string;
    saving: string;
    saved: string;

    // Errors
    errorPrefix: string;
    connFailedPrefix: string;
}

const zh: Translations = {
    appTitle: "SSH Forwarder",
    minimize: "最小化",
    maximize: "最大化",
    restore: "还原",
    close: "关闭",
    savedConnections: "已保存的连接",
    noSavedConnections: "暂无保存的连接",
    sshConfig: "SSH 连接配置",
    fillServerInfo: "填写服务器连接信息",
    hostAddress: "主机地址",
    username: "用户名",
    password: "密码",
    authMethod: "认证方式",
    passwordAuth: "密码认证",
    sshConfigOptional: "SSH 配置（可选）",
    sshConfigHint: "粘贴标准 SSH config 格式的配置",
    saveThisConnection: "保存此连接",
    connectionName: "连接名称",
    testConnection: "测试连接",
    testing: "测试中...",
    connect: "连接",
    connecting: "连接中...",
    sshProtocol: "支持 SSH-2 协议",
    connectedTo: "已连接到",
    user: "用户",
    forwardablePorts: "可转发的端口",
    startForward: "开启转发",
    stopForward: "停止转发",
    noForwardPorts: "服务器未配置可转发的端口",
    disconnect: "断开连接",
    rename: "重命名",
    delete: "删除",
    deleteConfirm: "确定要删除此连接吗？",
    ready: "就绪",
    connected: "已连接",
    disconnected: "未连接",
    upload: "上传",
    download: "下载",
    forwardingInfo: "端口转发通过SSH连接在您的本地机器和远程目标之间创建一个安全隧道。它仅转发发送到特定本地端口的流量，不会影响您的系统级网络设置。",
    testSuccess: "连接成功",
    testFailed: "测试失败",
    connRefused: "连接被拒绝: 目标主机未开放 SSH 服务",
    connTimeout: "连接超时: 无法到达目标主机",
    noRoute: "网络不可达: 无法路由到目标主机",
    authFailed: "认证失败: 用户名或密码错误",
    authNotSupported: "认证失败: 服务器不支持密码认证",
    connFailed: "连接失败",
    testingConnection: "正在测试连接...",
    settings: "设置",
    connectionSettings: "连接设置",
    agentPath: "远程 Agent 路径",
    agentPathHint: "服务器上 server-agent 二进制文件的路径",
    connectionTimeout: "连接超时",
    connectionTimeoutUnit: "秒",
    localBindAddress: "本地绑定地址",
    localOnly: "仅本机",
    lanShare: "局域网共享",
    autoReconnect: "自动重连",
    autoReconnectHint: "连接断开后自动尝试重新连接",
    appearance: "外观",
    theme: "主题",
    themeLight: "浅色",
    themeDark: "深色",
    language: "语言",
    resetDefaults: "恢复默认",
    cancel: "取消",
    save: "保存",
    saving: "保存中...",
    saved: "已保存",
    errorPrefix: "错误",
    connFailedPrefix: "连接失败",
};

const en: Translations = {
    appTitle: "SSH Forwarder",
    minimize: "Minimize",
    maximize: "Maximize",
    restore: "Restore",
    close: "Close",
    savedConnections: "Saved Connections",
    noSavedConnections: "No saved connections",
    sshConfig: "SSH Connection",
    fillServerInfo: "Enter server connection details",
    hostAddress: "Host",
    username: "Username",
    password: "Password",
    authMethod: "Auth Method",
    passwordAuth: "Password",
    sshConfigOptional: "SSH Config (optional)",
    sshConfigHint: "Paste standard SSH config format",
    saveThisConnection: "Save connection",
    connectionName: "Connection name",
    testConnection: "Test",
    testing: "Testing...",
    connect: "Connect",
    connecting: "Connecting...",
    sshProtocol: "SSH-2 Protocol",
    connectedTo: "Connected to",
    user: "User",
    forwardablePorts: "Forwardable Ports",
    startForward: "Start Forward",
    stopForward: "Stop Forward",
    noForwardPorts: "No forwardable ports configured on server",
    disconnect: "Disconnect",
    rename: "Rename",
    delete: "Delete",
    deleteConfirm: "Are you sure you want to delete this connection?",
    ready: "Ready",
    connected: "Connected",
    disconnected: "Disconnected",
    upload: "Up",
    download: "Down",
    forwardingInfo: "Port forwarding creates a secure tunnel from your local machine to the remote target. It only forwards traffic sent to the specific local port and does not affect your system-wide network settings.",
    testSuccess: "Connection successful",
    testFailed: "Test failed",
    connRefused: "Connection refused: SSH service not available",
    connTimeout: "Timeout: Host unreachable",
    noRoute: "No route to host",
    authFailed: "Auth failed: Invalid credentials",
    authNotSupported: "Auth failed: Password auth not supported",
    connFailed: "Connection failed",
    testingConnection: "Testing connection...",
    settings: "Settings",
    connectionSettings: "Connection",
    agentPath: "Remote Agent Path",
    agentPathHint: "Path to server-agent binary on remote server",
    connectionTimeout: "Timeout",
    connectionTimeoutUnit: "sec",
    localBindAddress: "Local Bind Address",
    localOnly: "Local only",
    lanShare: "LAN shared",
    autoReconnect: "Auto Reconnect",
    autoReconnectHint: "Automatically reconnect on disconnect",
    appearance: "Appearance",
    theme: "Theme",
    themeLight: "Light",
    themeDark: "Dark",
    language: "Language",
    resetDefaults: "Reset",
    cancel: "Cancel",
    save: "Save",
    saving: "Saving...",
    saved: "Saved",
    errorPrefix: "Error",
    connFailedPrefix: "Failed",
};

const translations: Record<Language, Translations> = { zh, en };

export function getTranslations(lang: Language): Translations {
    return translations[lang] || translations.zh;
}
