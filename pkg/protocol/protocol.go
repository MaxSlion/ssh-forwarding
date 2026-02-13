package protocol

const (
	MsgTypeHandshake = "handshake"
	MsgTypeConnect   = "connect"
)

type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type HandshakeRequest struct {
	Version string `json:"version"`
}

type PortConfig struct {
	Name        string `json:"name" yaml:"name"`
	Target      string `json:"target" yaml:"target"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Static      bool   `json:"static,omitempty" yaml:"static,omitempty"`
	LocalPort   int    `json:"local_port,omitempty" yaml:"local_port,omitempty"`
}

type HandshakeResponse struct {
	Version      string       `json:"version"`
	AllowedPorts []PortConfig `json:"allowed_ports"`
	Error        string       `json:"error,omitempty"`
}

type ConnectRequest struct {
	Target string `json:"target"`
}

type ConnectResponse struct {
    Success bool   `json:"success"`
    Error   string `json:"error,omitempty"`
}
