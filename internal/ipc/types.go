package ipc

import "encoding/json"

const (
	ProtocolVersion              = 3
	MaxTextActionBytes           = 512 << 10
	IncompatibleProtocolExitCode = 3
)

type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Status struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
	PID             int    `json:"pid"`
	SocketPath      string `json:"socket_path"`
	ConfigPath      string `json:"config_path"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
}

type Theme struct {
	Name   string `json:"name"`
	Colors any    `json:"colors"`
}
