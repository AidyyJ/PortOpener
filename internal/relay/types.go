package relay

import "net/http"

type ControlMessage struct {
	Type         string   `json:"type"`
	Token        string   `json:"token,omitempty"`
	ClientID     string   `json:"client_id,omitempty"`
	Version      string   `json:"version,omitempty"`
	TunnelID     string   `json:"tunnel_id,omitempty"`
	Protocol     string   `json:"protocol,omitempty"`
	Subdomain    string   `json:"subdomain,omitempty"`
	Allowlist    []string `json:"allowlist,omitempty"`
	LocalHost    string   `json:"local_host,omitempty"`
	LocalPort    int      `json:"local_port,omitempty"`
	ExternalPort int      `json:"external_port,omitempty"`
	ErrorCode    string   `json:"code,omitempty"`
	Message      string   `json:"message,omitempty"`
	Timestamp    string   `json:"timestamp,omitempty"`
}

type HTTPRequest struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Host        string      `json:"host"`
	Header      http.Header `json:"header"`
	RemoteAddr  string      `json:"remote_addr"`
	IsWebSocket bool        `json:"is_websocket"`
}

type HTTPResponse struct {
	Status int         `json:"status"`
	Header http.Header `json:"header"`
}

type UDPDatagram struct {
	RemoteAddr string `json:"remote_addr"`
	PayloadB64 string `json:"payload_b64"`
}
