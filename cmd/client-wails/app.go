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

// Global State
var (
	sshClient    *ssh.Client
	yamuxSession *yamux.Session
	// Map of localPort -> Listener
	listeners   map[string]net.Listener
	listenersMu sync.Mutex
	mu          sync.Mutex
)

// App struct
type App struct {
	ctx context.Context
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
	listeners = make(map[string]net.Listener)
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ConnectSSH establishes the SSH connection and handshake

// ConnectSSH establishes the SSH connection and handshake
func (a *App) ConnectSSH(req ConnectRequest) ConnectResponse {
	mu.Lock()
	defer mu.Unlock()

	// Cleanup previous
	a.disconnectLocked()

	// Reset metrics
	atomic.StoreUint64(&globalMetrics.BytesSent, 0)
	atomic.StoreUint64(&globalMetrics.BytesReceived, 0)

	err := connectSSH(req)
	if err != nil {
		return ConnectResponse{Success: false, Error: err.Error()}
	}

	resp, err := performHandshake()
	if err != nil {
		a.disconnectLocked()
		return ConnectResponse{Success: false, Error: err.Error()}
	}

	return ConnectResponse{Success: true, Config: resp}
}

// Disconnect closes the SSH session and all listeners
func (a *App) Disconnect() bool {
	mu.Lock()
	defer mu.Unlock()
	a.disconnectLocked()
	return true
}

func (a *App) disconnectLocked() {
	// Stop all listeners first
	listenersMu.Lock()
	for port, ln := range listeners {
		ln.Close()
		delete(listeners, port)
	}
	listenersMu.Unlock()

	if yamuxSession != nil {
		yamuxSession.Close()
		yamuxSession = nil
	}
	if sshClient != nil {
		sshClient.Close()
		sshClient = nil
	}
}

// StartForward starts a local listener that forwards traffic to the remote target
func (a *App) StartForward(localPort, target string) (string, error) {
	// If user specifically requested a port (not :0), check if we already have it tracked
	if localPort != ":0" && localPort != "0" {
		listenersMu.Lock()
		if _, exists := listeners[localPort]; exists {
			listenersMu.Unlock()
			return "", fmt.Errorf("Port already in use by this app")
		}
		listenersMu.Unlock()
	}

	ln, err := net.Listen("tcp", localPort)
	if err != nil {
		return "", fmt.Errorf("Failed to listen: %v", err)
	}
	
	boundAddr := ln.Addr().String()

	listenersMu.Lock()
	listeners[boundAddr] = ln
	listenersMu.Unlock()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // Listener closed
			}
			go handleForwarding(conn, target)
		}
	}()

	return boundAddr, nil
}

// StopForward stops the listener on the given local port
func (a *App) StopForward(localPort string) bool {
	listenersMu.Lock()
	defer listenersMu.Unlock()

	ln, exists := listeners[localPort]
	if !exists {
		return false
	}

	ln.Close()
	delete(listeners, localPort)
	return true
}

func handleForwarding(localConn net.Conn, target string) {
	defer localConn.Close()

	// Safety check
	if yamuxSession == nil {
		return
	}

	// Open Yamux stream
	stream, err := yamuxSession.Open()
	if err != nil {
		// Log error?
		return
	}
	defer stream.Close()

	// Send Connect Request
	req := protocol.ConnectRequest{Target: target}
	msg := protocol.Message{Type: protocol.MsgTypeConnect, Payload: req}
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		return
	}

	// Wait for Connect Response
	var resp protocol.ConnectResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return
	}

	if !resp.Success {
		// Target connection failed
		return
	}

	// Pipe data
	// Use CountedConn logic? No, global metrics tracks TOTAL bandwidth.
	// Since we wrap the SSH connection, all encrypted traffic is counted.
	// Payload traffic is inside SSH.
	// If the user wants "Application Throughput", we should count here.
	// If "Network Usage", we count at SSH level.
	// Let's count here (Application payload) for "Bandwidth" as it's more useful for "what is this forwarder doing".
	// The SSH wrapper counts overhead too.
	// Actually, let's stick to the plan: Wrap the SSH connection (CountedConn) to get TRUE network usage.
	
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
}

// TestConnectionResult holds the result of a connection test
type TestConnectionResult struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Latency   string `json:"latency,omitempty"`   // e.g. "120ms"
	SSHBanner string `json:"sshBanner,omitempty"` // server SSH banner
}

// TestConnection attempts SSH dial+auth, reports result, then disconnects.
// This follows industry standard: verify TCP reachability → SSH handshake → auth → disconnect.
func (a *App) TestConnection(req ConnectRequest) TestConnectionResult {
	start := time.Now()

	// 1. Build SSH config
	auths := []ssh.AuthMethod{}
	if req.Pass != "" {
		auths = append(auths, ssh.Password(req.Pass))
	}

	config := &ssh.ClientConfig{
		User:            req.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
		BannerCallback: func(message string) error {
			// Capture banner but don't block
			return nil
		},
	}

	// 2. Attempt SSH connection (covers TCP dial + SSH handshake + auth)
	client, err := ssh.Dial("tcp", req.Host, config)
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		return TestConnectionResult{
			Success: false,
			Error:   classifySSHError(err),
			Latency: latency,
		}
	}

	// 3. Grab server version banner
	banner := string(client.ServerVersion())

	// 4. Clean disconnect
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
	case contains(msg, "connection refused"):
		return "连接被拒绝: 目标主机未开放 SSH 服务"
	case contains(msg, "i/o timeout") || contains(msg, "deadline exceeded"):
		return "连接超时: 无法到达目标主机"
	case contains(msg, "no route to host"):
		return "网络不可达: 无法路由到目标主机"
	case contains(msg, "unable to authenticate") || contains(msg, "handshake failed"):
		return "认证失败: 用户名或密码错误"
	case contains(msg, "no supported methods remain"):
		return "认证失败: 服务器不支持密码认证"
	default:
		return fmt.Sprintf("连接失败: %s", msg)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// GetStatus returns the current connection status
func (a *App) GetStatus() bool {
	mu.Lock()
	defer mu.Unlock()
	return sshClient != nil && yamuxSession != nil && !yamuxSession.IsClosed()
}

// Helper functions (same as hybrid client)

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

func connectSSH(req ConnectRequest) error {
	auths := []ssh.AuthMethod{}
	if req.Pass != "" {
		auths = append(auths, ssh.Password(req.Pass))
	}
	// Add key support if needed

	config := &ssh.ClientConfig{
		User:            req.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	// Use custom dialer to wrap connection for metrics
	conn, err := net.DialTimeout("tcp", req.Host, config.Timeout)
	if err != nil {
		return err
	}

	// Wrap connection
	countedConn := &CountedConn{Conn: conn}

	// Establish SSH connection
	c, chans, reqs, err := ssh.NewClientConn(countedConn, req.Host, config)
	if err != nil {
		conn.Close()
		return err
	}
	client := ssh.NewClient(c, chans, reqs)
	
	sshClient = client

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
		agent = "./server-agent"
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
	yamuxSession = ysess
	return nil
}

func performHandshake() (*protocol.HandshakeResponse, error) {
	stream, err := yamuxSession.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := protocol.HandshakeRequest{Version: "2.0"}
	msg := protocol.Message{Type: protocol.MsgTypeHandshake, Payload: req}
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		return nil, err
	}

	var resp protocol.HandshakeResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("Server Error: %s", resp.Error)
	}
	return &resp, nil
}

type stdioRWC struct {
	io.Reader
	io.WriteCloser
}

func (c *stdioRWC) Close() error { return c.WriteCloser.Close() }
