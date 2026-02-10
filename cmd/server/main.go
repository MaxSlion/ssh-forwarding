package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"ssh-forwarder/pkg/protocol"

	"github.com/hashicorp/yamux"
	"gopkg.in/yaml.v3"
)

// ============================================================================
// Configuration
// ============================================================================

type ServerConfig struct {
	AllowedPorts    []protocol.PortConfig `yaml:"allowed_ports"`
	MaxStreams      int                   `yaml:"max_streams"`       // Max concurrent streams per session (default: 100)
	IdleTimeout     time.Duration         `yaml:"idle_timeout"`      // Idle timeout for connections (default: 5m)
	ConnectTimeout  time.Duration         `yaml:"connect_timeout"`   // Timeout for dialing targets (default: 10s)
	MetricsPort     int                   `yaml:"metrics_port"`      // Port for metrics endpoint (0 = disabled)
}

func defaultConfig() *ServerConfig {
	return &ServerConfig{
		AllowedPorts:   []protocol.PortConfig{},
		MaxStreams:     100,
		IdleTimeout:    5 * time.Minute,
		ConnectTimeout: 10 * time.Second,
		MetricsPort:    0,
	}
}

// ============================================================================
// Buffer Pool for zero-copy optimization
// ============================================================================

const bufferSize = 32 * 1024 // 32KB buffers

var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, bufferSize)
		return &buf
	},
}

func getBuffer() *[]byte {
	return bufferPool.Get().(*[]byte)
}

func putBuffer(buf *[]byte) {
	bufferPool.Put(buf)
}

// ============================================================================
// Metrics
// ============================================================================

type Metrics struct {
	ActiveStreams    int64
	TotalStreams     int64
	TotalBytes       int64
	HandshakeCount   int64
	ConnectCount     int64
	ConnectErrors    int64
	DeniedRequests   int64
}

var metrics = &Metrics{}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "# Server Metrics\n")
	fmt.Fprintf(w, "active_streams %d\n", atomic.LoadInt64(&m.ActiveStreams))
	fmt.Fprintf(w, "total_streams %d\n", atomic.LoadInt64(&m.TotalStreams))
	fmt.Fprintf(w, "total_bytes %d\n", atomic.LoadInt64(&m.TotalBytes))
	fmt.Fprintf(w, "handshake_count %d\n", atomic.LoadInt64(&m.HandshakeCount))
	fmt.Fprintf(w, "connect_count %d\n", atomic.LoadInt64(&m.ConnectCount))
	fmt.Fprintf(w, "connect_errors %d\n", atomic.LoadInt64(&m.ConnectErrors))
	fmt.Fprintf(w, "denied_requests %d\n", atomic.LoadInt64(&m.DeniedRequests))
}

// ============================================================================
// Server
// ============================================================================

type Server struct {
	session      *yamux.Session
	config       *ServerConfig
	activeCount  int64
	streamLimit  chan struct{}
}

func NewServer(session *yamux.Session, config *ServerConfig) *Server {
	return &Server{
		session:     session,
		config:      config,
		streamLimit: make(chan struct{}, config.MaxStreams),
	}
}

func main() {
	var stdioMode bool
	var configPath string
	flag.BoolVar(&stdioMode, "stdio", true, "Use stdin/stdout for transport")
	flag.StringVar(&configPath, "config", "server.yaml", "Path to server config")
	flag.Parse()

	// Configure logging to stderr
	log.SetOutput(os.Stderr)
	log.SetPrefix("[server-agent] ")

	if !stdioMode {
		log.Fatal("Only stdio mode is supported currently")
	}

	// Load Config
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config %s: %v. Using defaults.", configPath, err)
		cfg = defaultConfig()
	}

	// Start metrics server if configured
	if cfg.MetricsPort > 0 {
		go func() {
			addr := fmt.Sprintf("127.0.0.1:%d", cfg.MetricsPort)
			http.Handle("/metrics", metrics)
			log.Printf("Metrics server listening on %s", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Printf("Metrics server error: %v", err)
			}
		}()
	}

	// Stdio Transport
	conn := &stdioConn{
		Reader: os.Stdin,
		Writer: os.Stdout,
	}

	// Setup Yamux with optimized config
	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = true
	yamuxCfg.KeepAliveInterval = 30 * time.Second
	yamuxCfg.MaxStreamWindowSize = 1024 * 1024 // 1MB window for high throughput
	yamuxCfg.StreamOpenTimeout = 30 * time.Second
	yamuxCfg.StreamCloseTimeout = 5 * time.Minute

	session, err := yamux.Server(conn, yamuxCfg)
	if err != nil {
		log.Fatalf("Failed to create yamux server: %v", err)
	}

	server := NewServer(session, cfg)
	server.Serve()
}

func loadConfig(path string) (*ServerConfig, error) {
	// 1. Try path as is (relative to CWD)
	data, err := os.ReadFile(path)
	if err == nil {
		return parseConfig(data)
	}

	// 2. Try executable directory
	exePath, exeErr := os.Executable()
	if exeErr == nil {
		exeDir := filepath.Dir(exePath)
		fullPath := filepath.Join(exeDir, path)
		data, err = os.ReadFile(fullPath)
		if err == nil {
			log.Printf("Loaded config from executable dir: %s", fullPath)
			return parseConfig(data)
		}
	}

	return nil, err
}

func parseConfig(data []byte) (*ServerConfig, error) {
	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *Server) Serve() {
	for {
		stream, err := s.session.Accept()
		if err != nil {
			log.Printf("Session accept failed: %v", err)
			return
		}

		// Rate limit: try to acquire stream slot
		select {
		case s.streamLimit <- struct{}{}:
			// Got slot, proceed
			atomic.AddInt64(&metrics.ActiveStreams, 1)
			atomic.AddInt64(&metrics.TotalStreams, 1)
			go s.handleStream(stream)
		default:
			// At limit, reject
			log.Printf("Stream limit reached (%d), rejecting", s.config.MaxStreams)
			atomic.AddInt64(&metrics.DeniedRequests, 1)
			stream.Close()
		}
	}
}

func (s *Server) releaseStream() {
	<-s.streamLimit
	atomic.AddInt64(&metrics.ActiveStreams, -1)
}

func (s *Server) handleStream(stream net.Conn) {
	defer s.releaseStream()
	defer stream.Close()

	// Set idle timeout
	stream.SetDeadline(time.Now().Add(s.config.IdleTimeout))

	// Read Message (JSON)
	decoder := json.NewDecoder(stream)
	var msg protocol.Message
	if err := decoder.Decode(&msg); err != nil {
		log.Printf("Failed to decode message: %v", err)
		return
	}

	// Reset deadline after successful read
	stream.SetDeadline(time.Time{})

	switch msg.Type {
	case protocol.MsgTypeHandshake:
		atomic.AddInt64(&metrics.HandshakeCount, 1)
		s.handleHandshake(stream)
	case protocol.MsgTypeConnect:
		atomic.AddInt64(&metrics.ConnectCount, 1)
		payloadBytes, _ := json.Marshal(msg.Payload)
		var freq protocol.ConnectRequest
		if err := json.Unmarshal(payloadBytes, &freq); err != nil {
			log.Printf("Invalid connect payload: %v", err)
			return
		}
		s.handleConnect(stream, freq)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

func (s *Server) handleHandshake(stream net.Conn) {
	resp := protocol.HandshakeResponse{
		Version:      "2.0",
		AllowedPorts: s.config.AllowedPorts,
	}
	if err := json.NewEncoder(stream).Encode(resp); err != nil {
		log.Printf("Failed to send handshake response: %v", err)
	}
}

func (s *Server) handleConnect(stream net.Conn, req protocol.ConnectRequest) {
	// Validate Target
	allowed := false
	for _, p := range s.config.AllowedPorts {
		if p.Target == req.Target {
			allowed = true
			break
		}
	}

	resp := protocol.ConnectResponse{}
	if !allowed {
		resp.Success = false
		resp.Error = fmt.Sprintf("Target %s not allowed", req.Target)
		json.NewEncoder(stream).Encode(resp)
		log.Printf("Denied access to %s", req.Target)
		atomic.AddInt64(&metrics.DeniedRequests, 1)
		return
	}

	// Connect with timeout
	dialer := net.Dialer{Timeout: s.config.ConnectTimeout}
	targetConn, err := dialer.Dial("tcp", req.Target)
	if err != nil {
		resp.Success = false
		resp.Error = fmt.Sprintf("Dial failed: %v", err)
		json.NewEncoder(stream).Encode(resp)
		log.Printf("Failed to dial %s: %v", req.Target, err)
		atomic.AddInt64(&metrics.ConnectErrors, 1)
		return
	}

	resp.Success = true
	if err := json.NewEncoder(stream).Encode(resp); err != nil {
		targetConn.Close()
		return
	}

	defer targetConn.Close()

	// Proxy with buffer pool for zero-copy
	done := make(chan struct{}, 2)

	go func() {
		buf := getBuffer()
		defer putBuffer(buf)
		n, _ := io.CopyBuffer(targetConn, stream, *buf)
		atomic.AddInt64(&metrics.TotalBytes, n)
		if conn, ok := targetConn.(*net.TCPConn); ok {
			conn.CloseWrite()
		}
		done <- struct{}{}
	}()

	go func() {
		buf := getBuffer()
		defer putBuffer(buf)
		n, _ := io.CopyBuffer(stream, targetConn, *buf)
		atomic.AddInt64(&metrics.TotalBytes, n)
		stream.Close()
		done <- struct{}{}
	}()

	<-done
	<-done
	log.Printf("Closed connection to %s", req.Target)
}

// ============================================================================
// Stdio Connection Implementation
// ============================================================================

type stdioConn struct {
	io.Reader
	io.Writer
}

func (c *stdioConn) Close() error                       { return nil }
func (c *stdioConn) LocalAddr() net.Addr                { return &net.UnixAddr{Name: "stdio", Net: "stdio"} }
func (c *stdioConn) RemoteAddr() net.Addr               { return &net.UnixAddr{Name: "stdio", Net: "stdio"} }
func (c *stdioConn) SetDeadline(t time.Time) error      { return nil }
func (c *stdioConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *stdioConn) SetWriteDeadline(t time.Time) error { return nil }
