package relayclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AidyyJ/PortOpener/internal/httpbridge"
	"github.com/AidyyJ/PortOpener/internal/relay"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

type Config struct {
	URL             string
	Token           string
	ClientID        string
	HeartbeatPeriod time.Duration
	LocalBaseURL    string
	LocalHost       string
	LocalPort       int
}

type Client struct {
	url            string
	token          string
	clientID       string
	heartbeat      time.Duration
	localBase      string
	localHost      string
	localPort      int
	streamHandlers map[string]func(ctx context.Context, stream *yamux.Stream)
}

func New(cfg Config) *Client {
	clientID := cfg.ClientID
	if clientID == "" {
		clientID = uuid.NewString()
	}
	period := cfg.HeartbeatPeriod
	if period <= 0 {
		period = 10 * time.Second
	}
	return &Client{
		url:            cfg.URL,
		token:          cfg.Token,
		clientID:       clientID,
		heartbeat:      period,
		localBase:      strings.TrimRight(cfg.LocalBaseURL, "/"),
		localHost:      strings.TrimSpace(cfg.LocalHost),
		localPort:      cfg.LocalPort,
		streamHandlers: make(map[string]func(ctx context.Context, stream *yamux.Stream)),
	}
}

func (c *Client) AddStreamHandler(protocol string, handler func(ctx context.Context, stream *yamux.Stream)) {
	if strings.TrimSpace(protocol) == "" || handler == nil {
		return
	}
	if c.streamHandlers == nil {
		c.streamHandlers = make(map[string]func(ctx context.Context, stream *yamux.Stream))
	}
	c.streamHandlers[strings.ToLower(strings.TrimSpace(protocol))] = handler
}

func (c *Client) Run(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, c.url, &websocket.DialOptions{Subprotocols: []string{"binary"}})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	wsConn := websocket.NetConn(ctx, conn, websocket.MessageBinary)
	session, err := yamux.Client(wsConn, nil)
	if err != nil {
		return err
	}
	defer session.Close()

	control, err := session.OpenStream()
	if err != nil {
		return err
	}
	defer control.Close()

	if err := relay.WriteJSON(control, relay.ControlMessage{Type: "hello", Token: c.token, ClientID: c.clientID, Version: "dev"}); err != nil {
		return err
	}

	var response relay.ControlMessage
	if err := relay.ReadJSON(control, &response); err != nil {
		return err
	}
	if response.Type == "error" {
		return errors.New(response.Message)
	}
	if response.Type != "hello_ok" {
		return errors.New("unexpected relay response")
	}

	log.Printf("relay connected client_id=%s", c.clientID)

	ticker := time.NewTicker(c.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			if err := relay.WriteJSON(control, relay.ControlMessage{Type: "heartbeat", Timestamp: t.UTC().Format(time.RFC3339)}); err != nil {
				return err
			}
		}
	}
}

func (c *Client) RegisterHTTP(ctx context.Context, subdomain string, allowlist []string) error {
	conn, _, err := websocket.Dial(ctx, c.url, &websocket.DialOptions{Subprotocols: []string{"binary"}})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	wsConn := websocket.NetConn(ctx, conn, websocket.MessageBinary)
	session, err := yamux.Client(wsConn, nil)
	if err != nil {
		return err
	}
	defer session.Close()

	control, err := session.OpenStream()
	if err != nil {
		return err
	}
	defer control.Close()

	if err := relay.WriteJSON(control, relay.ControlMessage{Type: "hello", Token: c.token, ClientID: c.clientID, Version: "dev", TunnelID: uuid.NewString(), Protocol: "http", Subdomain: subdomain, Allowlist: allowlist, LocalHost: c.localHost, LocalPort: c.localPort}); err != nil {
		return err
	}

	var response relay.ControlMessage
	if err := relay.ReadJSON(control, &response); err != nil {
		return err
	}
	if response.Type == "error" {
		return errors.New(response.Message)
	}
	if response.Type != "hello_ok" {
		return errors.New("unexpected relay response")
	}

	if c.localBase == "" {
		return errors.New("local base url required")
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			stream, err := session.AcceptStream()
			if err != nil {
				errCh <- err
				return
			}
			handler := c.streamHandlers[strings.ToLower(strings.TrimSpace("http"))]
			if handler == nil {
				handler = c.handleHTTPStream
			}
			go handler(ctx, stream)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Client) handleHTTPStream(ctx context.Context, stream *yamux.Stream) {
	defer stream.Close()

	var req relay.HTTPRequest
	if err := relay.ReadJSON(stream, &req); err != nil {
		return
	}
	body, err := relay.ReadFrame(stream)
	if err != nil {
		return
	}

	if req.IsWebSocket {
		c.handleWebSocketStream(ctx, stream, req)
		return
	}

	respFrame, respBody := c.forwardHTTPRequest(ctx, req, body)
	_ = relay.WriteJSON(stream, respFrame)
	_ = relay.WriteFrame(stream, respBody)
}

func (c *Client) forwardHTTPRequest(ctx context.Context, req relay.HTTPRequest, body []byte) (relay.HTTPResponse, []byte) {
	base, err := url.Parse(c.localBase)
	if err != nil {
		return relay.HTTPResponse{Status: http.StatusBadGateway}, []byte("invalid local base url")
	}

	rel, err := url.Parse(req.Path)
	if err != nil {
		return relay.HTTPResponse{Status: http.StatusBadRequest}, []byte("invalid request path")
	}

	target := base.ResolveReference(rel)
	request, err := http.NewRequestWithContext(ctx, req.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		return relay.HTTPResponse{Status: http.StatusBadRequest}, []byte("invalid request")
	}
	request.Header = req.Header.Clone()
	request.Host = base.Host

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return relay.HTTPResponse{Status: http.StatusBadGateway}, []byte("upstream error")
	}

	respFrame, respBody, err := httpbridge.EncodeResponse(resp)
	if err != nil {
		return relay.HTTPResponse{Status: http.StatusBadGateway}, []byte("read response failed")
	}
	return respFrame, respBody
}

func (c *Client) handleWebSocketStream(ctx context.Context, stream *yamux.Stream, req relay.HTTPRequest) {
	wsURL := c.localBase
	if strings.HasPrefix(wsURL, "http://") {
		wsURL = "ws://" + strings.TrimPrefix(wsURL, "http://")
	} else if strings.HasPrefix(wsURL, "https://") {
		wsURL = "wss://" + strings.TrimPrefix(wsURL, "https://")
	}
	wsURL = strings.TrimRight(wsURL, "/") + req.Path

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: req.Header})
	if err != nil {
		_ = relay.WriteJSON(stream, relay.HTTPResponse{Status: http.StatusBadGateway})
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	_ = relay.WriteJSON(stream, relay.HTTPResponse{Status: http.StatusSwitchingProtocols, Header: http.Header{}})

	errCh := make(chan error, 2)
	go func() {
		for {
			msgType, data, err := conn.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			payload := append([]byte{byte(msgType)}, data...)
			if err := relay.WriteFrame(stream, payload); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			payload, err := relay.ReadFrame(stream)
			if err != nil {
				errCh <- err
				return
			}
			if len(payload) < 1 {
				errCh <- io.EOF
				return
			}
			msgType := websocket.MessageType(payload[0])
			data := payload[1:]
			if err := conn.Write(ctx, msgType, data); err != nil {
				errCh <- err
				return
			}
		}
	}()

	<-errCh
}
