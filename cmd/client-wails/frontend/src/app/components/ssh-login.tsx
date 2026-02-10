import { Settings, Terminal, ChevronDown, Loader2, Network, Check, Play, Square, Activity, Trash2, Edit2, MoreVertical, Info } from "lucide-react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
import { Textarea } from "./ui/textarea";
import { useState, useEffect, useRef } from "react";
import { connectV2, testConnection } from "../api";
import { WindowMinimise, WindowMaximise, WindowUnmaximise, WindowIsMaximised, Quit } from "../../../wailsjs/runtime/runtime";
import { Disconnect, StartForward, StopForward, GetMetrics, LoadSettings } from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";
import { SettingsModal } from "./settings-modal";
import { useSettings } from "../settings-context";

// Types
interface SavedConnection {
  name: string;
  host: string;
  port: string;
  username: string;
}

interface PortForward {
  name: string;
  target: string;
  description?: string;
}

// Storage helpers
const STORAGE_KEY = "ssh_saved_connections";

function loadSavedConnections(): SavedConnection[] {
  try {
    const data = localStorage.getItem(STORAGE_KEY);
    return data ? JSON.parse(data) : [];
  } catch {
    return [];
  }
}

function saveConnections(connections: SavedConnection[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(connections));
}

export function SSHLogin() {
  const { t, theme } = useSettings();

  // Connection form state
  const [host, setHost] = useState("127.0.0.1");
  const [port, setPort] = useState("22");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [sshConfig, setSshConfig] = useState("");
  const [saveConnection, setSaveConnection] = useState(false);
  const [connectionName, setConnectionName] = useState("");

  // UI state
  const [isLoading, setIsLoading] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  const [status, setStatus] = useState("");
  const [statusKey, setStatusKey] = useState(0);
  const [isMaximized, setIsMaximized] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);

  // Data state
  const [savedConnections, setSavedConnections] = useState<SavedConnection[]>([]);
  const [forwardedPorts, setForwardedPorts] = useState<PortForward[]>([]);

  // New features state
  const [forwardingStatus, setForwardingStatus] = useState<Record<string, string>>({}); // port.name -> boundAddress (empty if stopped)
  const [metrics, setMetrics] = useState<main.Metrics>(new main.Metrics());
  const [contextMenu, setContextMenu] = useState<{ x: number, y: number, conn: SavedConnection } | null>(null);
  const contextMenuRef = useRef<HTMLDivElement>(null);

  // Load saved connections on mount
  useEffect(() => {
    setSavedConnections(loadSavedConnections());
  }, []);

  const handleLogin = async () => {
    setIsLoading(true);
    setStatus(t.connecting);
    setStatusKey(k => k + 1);

    try {
      // Load current settings to get agentPath
      let agentPath = "";
      try {
        const s = await LoadSettings();
        agentPath = s.agentPath;
      } catch (e) {
        console.warn("Failed to load settings:", e);
      }

      const fullHost = `${host}:${port}`;
      const res = await connectV2({
        host: fullHost,
        username: username,
        password: password,
        agentPath: agentPath
      });

      if (res.success) {
        setIsConnected(true);
        setStatus(`${t.connectedTo} ${host}:${port}`);
        if (res.config?.allowed_ports) {
          setForwardedPorts(res.config.allowed_ports.map(p => ({
            name: p.name,
            target: p.target,
            description: p.description
          })));
        }

        if (saveConnection && connectionName) {
          const newConn: SavedConnection = {
            name: connectionName,
            host: host,
            port: port,
            username: username
          };
          const updated = [...savedConnections.filter(c => c.name !== connectionName), newConn];
          setSavedConnections(updated);
          saveConnections(updated);
        }
      } else {
        setStatus(`${t.errorPrefix}: ${res.error}`);
      }
    } catch (e) {
      setStatus(`${t.connFailedPrefix}: ${e}`);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSelectConnection = (conn: SavedConnection) => {
    setHost(conn.host);
    setPort(conn.port);
    setUsername(conn.username);
    setConnectionName(conn.name);
  };

  const handleTestConnection = async () => {
    setIsTesting(true);
    setStatus(t.testingConnection);
    setStatusKey(k => k + 1);

    try {
      const fullHost = `${host}:${port}`;
      const res = await testConnection({
        host: fullHost,
        username: username,
        password: password
      });

      if (res.success) {
        setStatus(`${t.testSuccess} (${res.latency}) - ${res.sshBanner || 'SSH Server'}`);
      } else {
        setStatus(`${res.error}${res.latency ? ` (${res.latency})` : ''}`);
      }
    } catch (e) {
      setStatus(`${t.testFailed}: ${e}`);
    } finally {
      setIsTesting(false);
    }
  };

  const handleSettings = () => {
    setIsSettingsOpen(true);
  };

  const handleMinimize = () => {
    WindowMinimise();
  };

  const handleMaximize = async () => {
    const maximized = await WindowIsMaximised();
    if (maximized) {
      WindowUnmaximise();
      setIsMaximized(false);
    } else {
      WindowMaximise();
      setIsMaximized(true);
    }
  };

  const handleClose = () => {
    Quit();
  };

  // Metrics polling
  useEffect(() => {
    let interval: number;
    if (isConnected) {
      interval = setInterval(async () => {
        try {
          const m = await GetMetrics();
          setMetrics(m);
        } catch (e) {
          console.error(e);
        }
      }, 1000);
    }
    return () => clearInterval(interval);
  }, [isConnected]);

  // Context menu click outside handler
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (contextMenuRef.current && !contextMenuRef.current.contains(event.target as Node)) {
        setContextMenu(null);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleDisconnect = async () => {
    await Disconnect();
    setIsConnected(false);
    setStatus(t.disconnected);
    setForwardingStatus({});
  };

  const handleToggleForward = async (port: PortForward) => {
    const currentAddr = forwardingStatus[port.name];

    if (currentAddr) {
      // Stop
      try {
        await StopForward(currentAddr); // Stop using the bound address
        setForwardingStatus(prev => {
          const next = { ...prev };
          delete next[port.name];
          return next;
        });
      } catch (e) {
        console.error("Failed to stop", e);
      }
    } else {
      // Start
      // Use ":0" to let OS pick a random port
      try {
        const boundAddr = await StartForward(":0", port.target);
        setForwardingStatus(prev => ({ ...prev, [port.name]: boundAddr }));
      } catch (e) {
        setStatus(`${t.errorPrefix}: ${e}`);
      }
    }
  };

  const handleContextMenu = (e: React.MouseEvent, conn: SavedConnection) => {
    e.preventDefault();
    setContextMenu({ x: e.clientX, y: e.clientY, conn });
  };

  const handleDeleteConnection = () => {
    if (contextMenu && confirm(t.deleteConfirm)) {
      const updated = savedConnections.filter(c => c.name !== contextMenu.conn.name);
      setSavedConnections(updated);
      saveConnections(updated);
      setContextMenu(null);
    }
  };

  const handleRenameConnection = () => {
    if (!contextMenu) return;
    const newName = prompt(t.connectionName, contextMenu.conn.name);
    if (newName && newName !== contextMenu.conn.name) {
      const updated = savedConnections.map(c =>
        c.name === contextMenu.conn.name ? { ...c, name: newName } : c
      );
      setSavedConnections(updated);
      saveConnections(updated);
      setContextMenu(null);
    }
  };

  // Dark mode aware colors
  const isDark = theme === "dark";

  return (
    <div className={`h-screen flex flex-col ${isDark ? 'bg-gray-900' : 'bg-slate-50'}`}>
      {/* 顶部工具栏 */}
      <div className={`h-12 border-b flex items-center justify-between px-4 flex-shrink-0 app-drag ${isDark ? 'bg-gray-800 border-gray-700' : 'bg-white border-slate-200'
        }`}>
        <div className="flex items-center gap-2">
          <Terminal className={`h-5 w-5 ${isDark ? 'text-gray-300' : 'text-slate-700'}`} />
          <span className={`font-semibold ${isDark ? 'text-gray-100' : 'text-slate-900'}`}>
            {t.appTitle}
          </span>
        </div>
        <div className="flex items-center app-no-drag">
          <Button
            variant="ghost"
            size="icon"
            onClick={handleSettings}
            className={`h-8 w-8 ${isDark ? 'hover:bg-gray-700' : 'hover:bg-slate-100'}`}
          >
            <Settings className={`h-4 w-4 ${isDark ? 'text-gray-400' : 'text-slate-600'}`} />
          </Button>
          <div className="flex items-center ml-2">
            <button
              onClick={handleMinimize}
              className={`w-11 h-8 flex items-center justify-center transition-colors ${isDark ? 'hover:bg-gray-700' : 'hover:bg-slate-100'
                }`}
              title={t.minimize}
            >
              <svg width="10" height="1" viewBox="0 0 10 1" className={isDark ? 'fill-gray-400' : 'fill-slate-600'}>
                <rect width="10" height="1" />
              </svg>
            </button>
            <button
              onClick={handleMaximize}
              className={`w-11 h-8 flex items-center justify-center transition-colors ${isDark ? 'hover:bg-gray-700' : 'hover:bg-slate-100'
                }`}
              title={isMaximized ? t.restore : t.maximize}
            >
              {isMaximized ? (
                <svg width="10" height="10" viewBox="0 0 10 10" className={`fill-none ${isDark ? 'stroke-gray-400' : 'stroke-slate-600'}`}>
                  <path d="M2 3h6v6H2V3z M3 3V1h6v6h-2" strokeWidth="1" />
                </svg>
              ) : (
                <svg width="10" height="10" viewBox="0 0 10 10" className={`fill-none ${isDark ? 'stroke-gray-400' : 'stroke-slate-600'}`}>
                  <rect x="0.5" y="0.5" width="9" height="9" strokeWidth="1" />
                </svg>
              )}
            </button>
            <button
              onClick={handleClose}
              className="w-11 h-8 flex items-center justify-center hover:bg-red-500 group transition-colors"
              title={t.close}
            >
              <svg width="10" height="10" viewBox="0 0 10 10" className={`group-hover:stroke-white ${isDark ? 'stroke-gray-400' : 'stroke-slate-600'}`}>
                <path d="M1 1l8 8M9 1l-8 8" strokeWidth="1.2" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      {/* 主内容区域 */}
      <div className="flex-1 flex overflow-hidden">
        {/* 左侧边栏 */}
        <div className={`w-60 border-r flex flex-col ${isDark ? 'bg-gray-800 border-gray-700' : 'bg-white border-slate-200'
          }`}>
          <div className={`p-4 border-b ${isDark ? 'border-gray-700' : 'border-slate-200'}`}>
            <h2 className={`font-medium text-sm ${isDark ? 'text-gray-200' : 'text-slate-900'}`}>
              {t.savedConnections}
            </h2>
          </div>
          <div className="flex-1 p-2 space-y-1 overflow-auto">
            {savedConnections.length === 0 ? (
              <div className={`text-xs px-3 py-4 text-center ${isDark ? 'text-gray-500' : 'text-slate-400'}`}>
                {t.noSavedConnections}
              </div>
            ) : (
              savedConnections.map((conn, idx) => (
                <button
                  key={idx}
                  onClick={() => handleSelectConnection(conn)}
                  onContextMenu={(e) => handleContextMenu(e, conn)}
                  className={`sidebar-item w-full text-left px-3 py-2 rounded transition-colors group relative ${isDark ? 'hover:bg-gray-700' : 'hover:bg-slate-100'
                    }`}
                >
                  <div className={`text-sm font-medium ${isDark ? 'text-gray-200' : 'text-slate-900'}`}>
                    {conn.name}
                  </div>
                  <div className={`text-xs ${isDark ? 'text-gray-500' : 'text-slate-500'}`}>
                    {conn.host}:{conn.port}
                  </div>
                  <div className="absolute right-2 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 transition-opacity">
                    <MoreVertical className={`h-4 w-4 ${isDark ? 'text-gray-400' : 'text-slate-400'}`} />
                  </div>
                </button>
              ))
            )}
          </div>
        </div>

        {/* 右侧主面板 */}
        <div className="flex-1 flex items-center justify-center p-6 overflow-auto">
          {isConnected ? (
            <div className="w-full max-w-2xl panel-enter">
              <div className={`rounded-lg shadow-sm border ${isDark ? 'bg-gray-800 border-gray-700' : 'bg-white border-slate-200'
                }`}>
                <div className={`px-6 py-4 border-b flex items-center justify-between ${isDark ? 'bg-green-900/30 border-gray-700' : 'bg-green-50 border-slate-200'
                  }`}>
                  <div className="flex items-center gap-3">
                    <div className={`p-2 rounded-full ${isDark ? 'bg-green-900/50' : 'bg-green-100'}`}>
                      <Check className="h-5 w-5 text-green-500" />
                    </div>
                    <div>
                      <h2 className={`font-semibold ${isDark ? 'text-gray-100' : 'text-slate-900'}`}>
                        {t.connectedTo} {host}:{port}
                      </h2>
                      <p className={`text-sm ${isDark ? 'text-gray-400' : 'text-slate-500'}`}>
                        {t.user}: {username}
                      </p>
                    </div>
                  </div>
                  <Button variant="destructive" size="sm" onClick={handleDisconnect} className="gap-2">
                    <Terminal className="h-4 w-4" />
                    {t.disconnect}
                  </Button>
                </div>
                <div className="p-6">
                  <h3 className={`font-medium mb-4 flex items-center gap-2 ${isDark ? 'text-gray-200' : 'text-slate-900'}`}>
                    <Network className="h-4 w-4" />
                    {t.forwardablePorts}
                  </h3>

                  <div className={`mb-4 p-3 rounded-lg text-xs flex items-start gap-2 ${isDark ? 'bg-blue-900/20 text-blue-200' : 'bg-blue-50 text-blue-700'}`}>
                    <Info className="h-4 w-4 flex-shrink-0 mt-0.5" />
                    <span className="leading-relaxed opacity-90">{t.forwardingInfo}</span>
                  </div>

                  {forwardedPorts.length > 0 ? (
                    <div className="space-y-3">
                      {forwardedPorts.map((port, idx) => {
                        const isRunning = !!forwardingStatus[port.name];
                        const boundAddr = forwardingStatus[port.name];
                        return (
                          <div key={idx} className={`port-item flex items-center justify-between p-4 rounded-lg border transition-all ${isRunning
                            ? (isDark ? 'bg-blue-900/20 border-blue-700/50' : 'bg-blue-50 border-blue-200')
                            : (isDark ? 'bg-gray-700/30 border-gray-600' : 'bg-slate-50 border-slate-200')
                            }`}>
                            <div className="flex-1">
                              <div className="flex items-center gap-2 mb-1">
                                <div className={`font-medium ${isDark ? 'text-gray-200' : 'text-slate-900'}`}>
                                  {port.name}
                                </div>
                                {isRunning && (
                                  <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${isDark ? 'bg-blue-900 text-blue-200' : 'bg-blue-100 text-blue-700'}`}>
                                    Active
                                  </span>
                                )}
                              </div>
                              <div className="flex items-center gap-2 text-sm font-mono">
                                <span className={isDark ? 'text-gray-400' : 'text-slate-500'}>Remote: {port.target}</span>
                                {isRunning && (
                                  <>
                                    <span className={isDark ? 'text-gray-600' : 'text-slate-400'}>→</span>
                                    <span className={`font-semibold ${isDark ? 'text-green-400' : 'text-green-600'}`}>
                                      Local: {boundAddr}
                                    </span>
                                  </>
                                )}
                              </div>
                              {port.description && (
                                <div className={`text-xs mt-1 ${isDark ? 'text-gray-500' : 'text-slate-400'}`}>
                                  {port.description}
                                </div>
                              )}
                            </div>
                            <Button
                              size="sm"
                              variant={isRunning ? "destructive" : "default"}
                              onClick={() => handleToggleForward(port)}
                              className={isRunning ? "" : "bg-blue-600 hover:bg-blue-700"}
                            >
                              {isRunning ? (
                                <>
                                  <Square className="h-3 w-3 mr-1.5 fill-current" />
                                  {t.stopForward}
                                </>
                              ) : (
                                <>
                                  <Play className="h-3 w-3 mr-1.5 fill-current" />
                                  {t.startForward}
                                </>
                              )}
                            </Button>
                          </div>
                        )
                      })}
                    </div>
                  ) : (
                    <div className={`text-center py-10 rounded-lg border border-dashed ${isDark ? 'border-gray-700 text-gray-500' : 'border-slate-300 text-slate-400'}`}>
                      {t.noForwardPorts}
                    </div>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="w-full max-w-2xl panel-enter">
              <div className={`card-hover rounded-lg shadow-sm border ${isDark ? 'bg-gray-800 border-gray-700' : 'bg-white border-slate-200'
                }`}>
                <div className={`px-6 py-4 border-b ${isDark ? 'bg-gray-750 border-gray-700' : 'bg-slate-50 border-slate-200'
                  }`}>
                  <h2 className={`font-semibold ${isDark ? 'text-gray-100' : 'text-slate-900'}`}>
                    {t.sshConfig}
                  </h2>
                  <p className={`text-sm mt-1 ${isDark ? 'text-gray-400' : 'text-slate-500'}`}>
                    {t.fillServerInfo}
                  </p>
                </div>

                <div className="p-6">
                  <div className="grid grid-cols-2 gap-6">
                    {/* 左列 */}
                    <div className="space-y-4">
                      <div className="space-y-2">
                        <Label htmlFor="host" className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                          {t.hostAddress}
                        </Label>
                        <div className="flex gap-2">
                          <Input
                            id="host"
                            type="text"
                            placeholder="example.com"
                            value={host}
                            onChange={(e) => setHost(e.target.value)}
                            className={`flex-1 h-9 ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100 placeholder:text-gray-500' : 'border-slate-300'}`}
                          />
                          <Input
                            id="port"
                            type="text"
                            placeholder="22"
                            value={port}
                            onChange={(e) => setPort(e.target.value)}
                            className={`w-20 h-9 text-center ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100 placeholder:text-gray-500' : 'border-slate-300'}`}
                          />
                        </div>
                      </div>

                      <div className="space-y-2">
                        <Label htmlFor="username" className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                          {t.username}
                        </Label>
                        <Input
                          id="username"
                          type="text"
                          placeholder="root"
                          value={username}
                          onChange={(e) => setUsername(e.target.value)}
                          className={`h-9 ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100 placeholder:text-gray-500' : 'border-slate-300'}`}
                        />
                      </div>

                      <div className="space-y-2">
                        <Label htmlFor="password" className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                          {t.password}
                        </Label>
                        <Input
                          id="password"
                          type="password"
                          placeholder="••••••••"
                          value={password}
                          onChange={(e) => setPassword(e.target.value)}
                          className={`h-9 ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100 placeholder:text-gray-500' : 'border-slate-300'}`}
                        />
                      </div>

                      <div className="space-y-2">
                        <Label className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                          {t.authMethod}
                        </Label>
                        <Button variant="outline" className={`w-full justify-between h-9 ${isDark ? 'bg-gray-700 border-gray-600 text-gray-200' : ''
                          }`}>
                          {t.passwordAuth}
                          <ChevronDown className={`h-4 w-4 ${isDark ? 'text-gray-400' : 'text-slate-500'}`} />
                        </Button>
                      </div>
                    </div>

                    {/* 右列 */}
                    <div className="space-y-2">
                      <Label htmlFor="sshConfig" className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                        {t.sshConfigOptional}
                      </Label>
                      <Textarea
                        id="sshConfig"
                        placeholder={"Host example\n  HostName example.com\n  Port 22\n  User root"}
                        value={sshConfig}
                        onChange={(e) => setSshConfig(e.target.value)}
                        className={`min-h-[180px] font-mono text-xs resize-none leading-relaxed ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100 placeholder:text-gray-500' : 'border-slate-300'
                          }`}
                      />
                      <p className={`text-xs ${isDark ? 'text-gray-500' : 'text-slate-500'}`}>
                        {t.sshConfigHint}
                      </p>
                    </div>
                  </div>

                  {/* 底部操作栏 */}
                  <div className={`mt-6 pt-4 border-t flex items-center justify-between ${isDark ? 'border-gray-700' : 'border-slate-200'
                    }`}>
                    <div className="flex items-center gap-3">
                      <input
                        type="checkbox"
                        id="saveConnection"
                        checked={saveConnection}
                        onChange={(e) => setSaveConnection(e.target.checked)}
                        className={`rounded ${isDark ? 'border-gray-600' : 'border-slate-300'}`}
                      />
                      <Label htmlFor="saveConnection" className={`text-sm cursor-pointer ${isDark ? 'text-gray-400' : 'text-slate-600'}`}>
                        {t.saveThisConnection}
                      </Label>
                      {saveConnection && (
                        <Input
                          type="text"
                          placeholder={t.connectionName}
                          value={connectionName}
                          onChange={(e) => setConnectionName(e.target.value)}
                          className={`h-8 w-32 text-sm ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100' : 'border-slate-300'}`}
                        />
                      )}
                    </div>
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        className={`px-4 h-9 ${isDark ? 'border-gray-600 text-gray-300 hover:bg-gray-700' : ''}`}
                        onClick={handleTestConnection}
                        disabled={isTesting || isLoading}
                      >
                        {isTesting && <Loader2 className="h-4 w-4 animate-spin mr-1" />}
                        {isTesting ? t.testing : t.testConnection}
                      </Button>
                      <Button
                        onClick={handleLogin}
                        disabled={isLoading || isTesting}
                        className="px-6 h-9 bg-blue-600 hover:bg-blue-700 text-white flex gap-2 btn-primary-glow"
                      >
                        {isLoading && <Loader2 className="h-4 w-4 animate-spin" />}
                        {isLoading ? t.connecting : t.connect}
                      </Button>
                    </div>
                  </div>
                </div>
              </div>

              {/* 底部提示 */}
              <div className={`mt-3 flex items-center justify-between text-xs ${isDark ? 'text-gray-500' : 'text-slate-500'}`}>
                <span>{t.sshProtocol}</span>
                <span>v1.0.0</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* 底部状态栏 */}
      <div className={`h-7 flex items-center justify-between px-4 text-xs flex-shrink-0 ${isDark ? 'bg-gray-950 text-gray-400' : 'bg-slate-800 text-slate-300'
        }`}>
        <span key={statusKey} className="status-text">{status || t.ready}</span>
        <div className="flex items-center gap-4">
          {isConnected && (
            <div className="flex items-center gap-3 font-mono">
              <span className="flex items-center gap-1">
                <Activity className="h-3 w-3 text-green-500" />
                ↑ {formatBytes(metrics?.bytesSent || 0)}
              </span>
              <span className="flex items-center gap-1">
                <Activity className="h-3 w-3 text-blue-500" />
                ↓ {formatBytes(metrics?.bytesReceived || 0)}
              </span>
            </div>
          )}
          <span>{isConnected ? t.connected : t.disconnected}</span>
        </div>
      </div>

      {/* Context Menu */}
      {contextMenu && (
        <div
          ref={contextMenuRef}
          className={`fixed z-50 rounded-md shadow-lg border p-1 w-32 ${isDark ? 'bg-gray-800 border-gray-700' : 'bg-white border-slate-200'}`}
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          <button
            onClick={handleRenameConnection}
            className={`w-full text-left px-3 py-1.5 text-xs rounded flex items-center gap-2 ${isDark ? 'hover:bg-gray-700 text-gray-200' : 'hover:bg-slate-100 text-slate-700'}`}
          >
            <Edit2 className="h-3 w-3" />
            {t.rename}
          </button>
          <button
            onClick={handleDeleteConnection}
            className={`w-full text-left px-3 py-1.5 text-xs rounded flex items-center gap-2 text-red-500 ${isDark ? 'hover:bg-gray-700/50' : 'hover:bg-red-50'}`}
          >
            <Trash2 className="h-3 w-3" />
            {t.delete}
          </button>
        </div>
      )}

      {/* 设置弹窗 */}
      <SettingsModal isOpen={isSettingsOpen} onClose={() => setIsSettingsOpen(false)} />
    </div>
  );
}

function formatBytes(bytes: number, decimals = 2) {
  if (!+bytes) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}