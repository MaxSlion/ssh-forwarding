package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
	"ssh-forwarder/pkg/protocol"
)

// ============================================================================
// App — all connection state lives here (no globals)
// ============================================================================

type App struct {
	ctx context.Context

	mu             sync.Mutex
	sshClient      *ssh.Client
	yamuxSession   *yamux.Session
	listeners      map[string]net.Listener
	lastConnectReq *ConnectRequest              // saved for auto-reconnect
	activeForwards map[string]string             // boundAddr -> target (for reconnect)
	heartbeatStop  chan struct{}                  // signal to stop heartbeat goroutine
}

// ── Request / Response types ────────────────────────────────────────────────

type ConnectRequest struct {
	Host      string `json:"host"`
	User      string `json:"username"`
	Pass      string `json:"password"`
	KeyPath   string `json:"keyPath"`
	AgentPath string `json:"agentPath"`
}

type ConnectResponse struct {
	Success bool                        `json:"success"`
	Error   string                      `json:"error,omitempty"`
	Config  *protocol.HandshakeResponse `json:"config,omitempty"`
}

type TestConnectionResult struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Latency   string `json:"latency,omitempty"`
	SSHBanner string `json:"sshBanner,omitempty"`
}

// ── Lifecycle ───────────────────────────────────────────────────────────────

func NewApp() *App {
	return &App{
		listeners:      make(map[string]net.Listener),
		activeForwards: make(map[string]string),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ── Session helpers ─────────────────────────────────────────────────────────

// getSession safely returns current yamux session (may be nil)
func (a *App) getSession() *yamux.Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.yamuxSession
}

// emitEvent sends an event to the frontend
func (a *App) emitEvent(name string, data interface{}) {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, name, data)
	}
}

// ============================================================================
// ConnectSSH — main connection entry point
// ============================================================================

func (a *App) ConnectSSH(req ConnectRequest) ConnectResponse {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Cleanup previous
	a.disconnectLocked()

	// Reset metrics
	atomic.StoreUint64(&globalMetrics.BytesSent, 0)
	atomic.StoreUint64(&globalMetrics.BytesReceived, 0)

	settings := a.loadSettingsUnlocked()

	err := a.connectSSH(req, settings)
	if err != nil {
		return ConnectResponse{Success: false, Error: err.Error()}
	}

	resp, err := a.performHandshake()
	if err != nil {
		a.disconnectLocked()
		return ConnectResponse{Success: false, Error: err.Error()}
	}

	// Save for auto-reconnect
	reqCopy := req
	a.lastConnectReq = &reqCopy

	// Start heartbeat
	a.heartbeatStop = make(chan struct{})
	go a.heartbeatLoop(a.heartbeatStop)

	return ConnectResponse{Success: true, Config: resp}
}

// ============================================================================
// Disconnect
// ============================================================================

func (a *App) Disconnect() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastConnectReq = nil // clear to prevent auto-reconnect
	a.disconnectLocked()
	return true
}

func (a *App) disconnectLocked() {
	// Stop heartbeat
	if a.heartbeatStop != nil {
		close(a.heartbeatStop)
		a.heartbeatStop = nil
	}

	// Stop all listeners
	for addr, ln := range a.listeners {
		ln.Close()
		delete(a.listeners, addr)
	}

	if a.yamuxSession != nil {
		a.yamuxSession.Close()
		a.yamuxSession = nil
	}
	if a.sshClient != nil {
		a.sshClient.Close()
		a.sshClient = nil
	}
}

// ============================================================================
// Port Forwarding
// ============================================================================

func (a *App) StartForward(localPort, target string) (string, error) {
	settings := a.LoadSettings()
	bindAddr := localPort
	if localPort == ":0" || localPort == "0" {
		bindAddr = settings.LocalBindAddress + ":0"
	} else {
		port := strings.TrimPrefix(localPort, ":")
		bindAddr = net.JoinHostPort(settings.LocalBindAddress, port)
	}

	a.mu.Lock()
	if localPort != ":0" && localPort != "0" {
		for _, ln := range a.listeners {
			if ln.Addr().String() == bindAddr {
				a.mu.Unlock()
				return "", fmt.Errorf("port already in use by this app")
			}
		}
	}
	a.mu.Unlock()

	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return "", fmt.Errorf("failed to listen on %s: %v", bindAddr, err)
	}

	boundAddr := ln.Addr().String()

	a.mu.Lock()
	a.listeners[boundAddr] = ln
	a.activeForwards[boundAddr] = target // track for reconnect
	a.mu.Unlock()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // Listener closed
			}

			// TCP tuning on accepted connection
			if tc, ok := conn.(*net.TCPConn); ok {
				tc.SetKeepAlive(true)
				tc.SetKeepAlivePeriod(30 * time.Second)
				tc.SetNoDelay(true)
			}

			go a.handleForwarding(conn, target)
		}
	}()

	log.Printf("[forward] listening on %s -> %s", boundAddr, target)
	return boundAddr, nil
}

func (a *App) StopForward(localPort string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	ln, exists := a.listeners[localPort]
	if !exists {
		return false
	}

	ln.Close()
	delete(a.listeners, localPort)
	delete(a.activeForwards, localPort)
	log.Printf("[forward] stopped %s", localPort)
	return true
}

// handleForwarding proxies data with proper half-close (P0) and error logging
func (a *App) handleForwarding(localConn net.Conn, target string) {
	defer localConn.Close()

	session := a.getSession()
	if session == nil {
		log.Printf("[forward] no active session for %s", target)
		return
	}

	// Open Yamux stream
	stream, err := session.Open()
	if err != nil {
		log.Printf("[forward] failed to open stream for %s: %v", target, err)
		a.emitEvent("forward-error", map[string]string{
			"target": target, "error": fmt.Sprintf("stream open: %v", err),
		})
		return
	}
	defer stream.Close()

	// Send Connect Request
	payload, _ := json.Marshal(protocol.ConnectRequest{Target: target})
	msg := protocol.Message{Type: protocol.MsgTypeConnect, Payload: payload}
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		log.Printf("[forward] connect encode error for %s: %v", target, err)
		return
	}

	// Wait for Connect Response
	var resp protocol.ConnectResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		log.Printf("[forward] connect decode error for %s: %v", target, err)
		return
	}
	if !resp.Success {
		log.Printf("[forward] remote denied %s: %s", target, resp.Error)
		a.emitEvent("forward-error", map[string]string{
			"target": target, "error": resp.Error,
		})
		return
	}

	// ── Bidirectional copy with proper half-close ────────────────────
	done := make(chan struct{}, 2)

	// local → remote
	go func() {
		io.Copy(stream, localConn)
		// Signal remote: local finished sending
		stream.Close() // yamux stream Close acts as half-close
		done <- struct{}{}
	}()

	// remote → local
	go func() {
		io.Copy(localConn, stream)
		// Signal local app: remote finished sending
		if tc, ok := localConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}

// ============================================================================
// Heartbeat — detect dead connections early
// ============================================================================

func (a *App) heartbeatLoop(stop chan struct{}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	failCount := 0

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			session := a.getSession()
			if session == nil || session.IsClosed() {
				log.Printf("[heartbeat] session gone, triggering reconnect")
				a.emitEvent("connection-lost", nil)
				go a.tryReconnect()
				return
			}

			// Ping via a short-lived stream
			if err := a.sendPing(session); err != nil {
				failCount++
				log.Printf("[heartbeat] ping failed (%d/3): %v", failCount, err)
				if failCount >= 3 {
					log.Printf("[heartbeat] 3 consecutive failures, triggering reconnect")
					a.emitEvent("connection-lost", nil)
					go a.tryReconnect()
					return
				}
			} else {
				failCount = 0
			}
		}
	}
}

func (a *App) sendPing(session *yamux.Session) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	stream.SetDeadline(time.Now().Add(5 * time.Second))

	payload, _ := json.Marshal(protocol.PingRequest{Timestamp: time.Now().UnixMilli()})
	msg := protocol.Message{Type: protocol.MsgTypePing, Payload: payload}
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		return err
	}

	var pong protocol.PongResponse
	if err := json.NewDecoder(stream).Decode(&pong); err != nil {
		return err
	}
	return nil
}

// ============================================================================
// Auto-reconnect with exponential backoff
// ============================================================================

func (a *App) tryReconnect() {
	a.mu.Lock()
	req := a.lastConnectReq
	if req == nil {
		a.mu.Unlock()
		return // user explicitly disconnected, don't reconnect
	}
	reqCopy := *req

	// Snapshot active forwards before disconnecting
	forwards := make(map[string]string)
	for addr, target := range a.activeForwards {
		forwards[addr] = target
	}

	a.disconnectLocked()
	a.mu.Unlock()

	settings := a.LoadSettings()
	if !settings.AutoReconnect {
		log.Printf("[reconnect] auto-reconnect disabled in settings")
		a.emitEvent("connection-status", map[string]string{"status": "disconnected"})
		return
	}

	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		delay := time.Duration(attempt*attempt) * time.Second // 1s, 4s, 9s, 16s, 25s
		log.Printf("[reconnect] attempt %d/%d in %v...", attempt, maxAttempts, delay)
		a.emitEvent("reconnecting", map[string]interface{}{
			"attempt": attempt, "maxAttempts": maxAttempts, "delay": delay.String(),
		})
		time.Sleep(delay)

		a.mu.Lock()
		if a.lastConnectReq == nil {
			// User manually disconnected during backoff
			a.mu.Unlock()
			return
		}

		err := a.connectSSH(reqCopy, settings)
		if err != nil {
			a.mu.Unlock()
			log.Printf("[reconnect] attempt %d failed: %v", attempt, err)
			continue
		}

		_, err = a.performHandshake()
		if err != nil {
			a.disconnectLocked()
			a.mu.Unlock()
			log.Printf("[reconnect] handshake failed: %v", err)
			continue
		}

		// Restart heartbeat
		a.heartbeatStop = make(chan struct{})
		go a.heartbeatLoop(a.heartbeatStop)

		a.mu.Unlock()

		// Restore port forwards
		restoredCount := 0
		for _, target := range forwards {
			if _, err := a.StartForward(":0", target); err == nil {
				restoredCount++
			} else {
				log.Printf("[reconnect] failed to restore forward to %s: %v", target, err)
			}
		}

		log.Printf("[reconnect] success! Restored %d/%d forwards", restoredCount, len(forwards))
		a.emitEvent("reconnected", map[string]interface{}{
			"restored": restoredCount, "total": len(forwards),
		})
		return
	}

	log.Printf("[reconnect] all %d attempts failed", maxAttempts)
	a.emitEvent("connection-status", map[string]string{
		"status": "failed", "error": "auto-reconnect failed after max attempts",
	})
}

// ============================================================================
// TestConnection
// ============================================================================

func (a *App) TestConnection(req ConnectRequest) TestConnectionResult {
	start := time.Now()

	settings := a.LoadSettings()
	timeout := time.Duration(settings.ConnectionTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	auths := []ssh.AuthMethod{}
	if req.Pass != "" {
		auths = append(auths, ssh.Password(req.Pass))
	}

	config := &ssh.ClientConfig{
		User:            req.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
		BannerCallback:  func(message string) error { return nil },
	}

	client, err := ssh.Dial("tcp", req.Host, config)
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		return TestConnectionResult{
			Success: false, Error: classifySSHError(err), Latency: latency,
		}
	}

	banner := string(client.ServerVersion())
	client.Close()

	return TestConnectionResult{Success: true, Latency: latency, SSHBanner: banner}
}

// GetStatus returns the current connection status
func (a *App) GetStatus() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sshClient != nil && a.yamuxSession != nil && !a.yamuxSession.IsClosed()
}

// ============================================================================
// SSH/Yamux internals
// ============================================================================

// CountedConn wraps net.Conn to track bytes
type CountedConn struct {
	net.Conn
}

func (c *CountedConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	addReceived(uint64(n))
	return
}

func (c *CountedConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	addSent(uint64(n))
	return
}

func (a *App) connectSSH(req ConnectRequest, settings AppSettings) error {
	auths := []ssh.AuthMethod{}
	if req.Pass != "" {
		auths = append(auths, ssh.Password(req.Pass))
	}

	timeout := time.Duration(settings.ConnectionTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	config := &ssh.ClientConfig{
		User:            req.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	conn, err := net.DialTimeout("tcp", req.Host, config.Timeout)
	if err != nil {
		return err
	}

	// TCP tuning on SSH connection
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(30 * time.Second)
		tc.SetNoDelay(true)
	}

	countedConn := &CountedConn{Conn: conn}

	c, chans, reqs, err := ssh.NewClientConn(countedConn, req.Host, config)
	if err != nil {
		conn.Close()
		return err
	}
	client := ssh.NewClient(c, chans, reqs)
	a.sshClient = client

	session, err := client.NewSession()
	if err != nil {
		return err
	}

	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()
	go io.Copy(os.Stderr, stderr)

	agent := req.AgentPath
	if agent == "" {
		agent = settings.AgentPath
		if agent == "" {
			agent = "/server-agent"
		}
	}
	cmd := fmt.Sprintf("%s --stdio", agent)
	if err := session.Start(cmd); err != nil {
		return err
	}

	rwConn := &stdioRWC{Reader: stdout, WriteCloser: stdin}

	// Yamux config aligned with server (1MB window)
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = 15 * time.Second
	cfg.MaxStreamWindowSize = 1024 * 1024 // 1MB — match server

	ysess, err := yamux.Client(rwConn, cfg)
	if err != nil {
		return err
	}
	a.yamuxSession = ysess
	return nil
}

func (a *App) performHandshake() (*protocol.HandshakeResponse, error) {
	stream, err := a.yamuxSession.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	payload, _ := json.Marshal(protocol.HandshakeRequest{Version: "2.0"})
	msg := protocol.Message{Type: protocol.MsgTypeHandshake, Payload: payload}
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		return nil, err
	}

	var resp protocol.HandshakeResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}
	return &resp, nil
}

// ============================================================================
// Helpers
// ============================================================================

func classifySSHError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return "连接被拒绝: 目标主机未开放 SSH 服务"
	case strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "deadline exceeded"):
		return "连接超时: 无法到达目标主机"
	case strings.Contains(msg, "no route to host"):
		return "网络不可达: 无法路由到目标主机"
	case strings.Contains(msg, "unable to authenticate") || strings.Contains(msg, "handshake failed"):
		return "认证失败: 用户名或密码错误"
	case strings.Contains(msg, "no supported methods remain"):
		return "认证失败: 服务器不支持密码认证"
	default:
		return fmt.Sprintf("连接失败: %s", msg)
	}
}

func (a *App) loadSettingsUnlocked() AppSettings {
	path := settingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultSettings()
	}
	settings := defaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultSettings()
	}
	return settings
}

type stdioRWC struct {
	io.Reader
	io.WriteCloser
}

func (c *stdioRWC) Close() error { return c.WriteCloser.Close() }
