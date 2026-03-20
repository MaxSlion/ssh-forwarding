package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"
	"golang.org/x/crypto/ssh"
	"ssh-forwarder/pkg/protocol"
)

// App struct holds all connection state (Fix #1: no more global mutable state)
type App struct {
	ctx          context.Context
	mu           sync.Mutex // single lock for all state
	sshClient    *ssh.Client
	yamuxSession *yamux.Session
	listeners    map[string]net.Listener
}

// ConnectRequest holds SSH connection details
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

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		listeners: make(map[string]net.Listener),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// getSession safely returns the current yamux session (Fix #6)
func (a *App) getSession() *yamux.Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.yamuxSession
}

// ConnectSSH establishes the SSH connection and handshake
func (a *App) ConnectSSH(req ConnectRequest) ConnectResponse {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Cleanup previous
	a.disconnectLocked()

	// Reset metrics
	atomic.StoreUint64(&globalMetrics.BytesSent, 0)
	atomic.StoreUint64(&globalMetrics.BytesReceived, 0)

	// Fix #3: Load settings for timeout
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

	return ConnectResponse{Success: true, Config: resp}
}

// Disconnect closes the SSH session and all listeners
func (a *App) Disconnect() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.disconnectLocked()
	return true
}

func (a *App) disconnectLocked() {
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

// StartForward starts a local listener that forwards traffic to the remote target
func (a *App) StartForward(localPort, target string) (string, error) {
	// Fix #4: Use localBindAddress from settings
	settings := a.LoadSettings()
	bindAddr := localPort
	if localPort == ":0" || localPort == "0" {
		bindAddr = settings.LocalBindAddress + ":0"
	} else {
		// localPort is like ":2222", prepend bind address
		port := strings.TrimPrefix(localPort, ":")
		bindAddr = net.JoinHostPort(settings.LocalBindAddress, port)
	}

	a.mu.Lock()
	// Check if port already tracked
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
	a.mu.Unlock()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // Listener closed
			}
			go a.handleForwarding(conn, target)
		}
	}()

	return boundAddr, nil
}

// StopForward stops the listener on the given local port
func (a *App) StopForward(localPort string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	ln, exists := a.listeners[localPort]
	if !exists {
		return false
	}

	ln.Close()
	delete(a.listeners, localPort)
	return true
}

// handleForwarding proxies data between the local connection and remote target
func (a *App) handleForwarding(localConn net.Conn, target string) {
	defer localConn.Close()

	// Fix #6: safely get session reference
	session := a.getSession()
	if session == nil {
		return
	}

	// Open Yamux stream
	stream, err := session.Open()
	if err != nil {
		return
	}
	defer stream.Close()

	// Send Connect Request (Fix #5: use json.RawMessage)
	payload, _ := json.Marshal(protocol.ConnectRequest{Target: target})
	msg := protocol.Message{Type: protocol.MsgTypeConnect, Payload: payload}
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		return
	}

	// Wait for Connect Response
	var resp protocol.ConnectResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return
	}

	if !resp.Success {
		return
	}

	// Fix #2: Pipe data and wait for BOTH directions to complete
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(stream, localConn) // Upload
		done <- struct{}{}
	}()
	go func() {
		io.Copy(localConn, stream) // Download
		done <- struct{}{}
	}()
	<-done
	<-done // ← Fix: wait for both directions
}

// TestConnectionResult holds the result of a connection test
type TestConnectionResult struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Latency   string `json:"latency,omitempty"`
	SSHBanner string `json:"sshBanner,omitempty"`
}

// TestConnection attempts SSH dial+auth, reports result, then disconnects.
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: implement known_hosts
		Timeout:         timeout,
		BannerCallback: func(message string) error {
			return nil
		},
	}

	client, err := ssh.Dial("tcp", req.Host, config)
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		return TestConnectionResult{
			Success: false,
			Error:   classifySSHError(err),
			Latency: latency,
		}
	}

	banner := string(client.ServerVersion())
	client.Close()

	return TestConnectionResult{
		Success:   true,
		Latency:   latency,
		SSHBanner: banner,
	}
}

// classifySSHError returns a user-friendly error message
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

// GetStatus returns the current connection status
func (a *App) GetStatus() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sshClient != nil && a.yamuxSession != nil && !a.yamuxSession.IsClosed()
}

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

// connectSSH establishes the SSH connection (Fix #3: uses settings for timeout)
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: implement known_hosts
		Timeout:         timeout,
	}

	conn, err := net.DialTimeout("tcp", req.Host, config.Timeout)
	if err != nil {
		return err
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
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	ysess, err := yamux.Client(rwConn, cfg)
	if err != nil {
		return err
	}
	a.yamuxSession = ysess
	return nil
}

// performHandshake sends handshake request and receives port config (Fix #5: json.RawMessage)
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

// loadSettingsUnlocked reads settings without acquiring a.mu (caller must hold lock)
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
