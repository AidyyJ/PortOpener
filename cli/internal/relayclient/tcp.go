package relayclient

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/AidyyJ/PortOpener/internal/relay"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

func (c *Client) HandleTCPStream(ctx context.Context, stream *yamux.Stream) {
	defer stream.Close()
	var msg relay.ControlMessage
	if err := relay.ReadJSON(stream, &msg); err != nil {
		return
	}
	if msg.Type != "tcp_open" {
		return
	}
	address := c.localHost
	if address == "" {
		address = "localhost"
	}
	port := c.localPort
	if port == 0 {
		port = msg.LocalPort
	}
	if port == 0 {
		return
	}
	if !strings.Contains(address, ":") {
		address = net.JoinHostPort(address, strconv.Itoa(port))
	}
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return
	}
	defer conn.Close()

	copyErr := make(chan error, 2)
	go func() {
		_, err := io.Copy(stream, conn)
		copyErr <- err
	}()
	go func() {
		_, err := io.Copy(conn, stream)
		copyErr <- err
	}()
	<-copyErr
}

func (c *Client) RegisterTCP(ctx context.Context, externalPort int) error {
	if externalPort == 0 {
		return nil
	}
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

	if err := relay.WriteJSON(control, relay.ControlMessage{
		Type:         "hello",
		Token:        c.token,
		ClientID:     c.clientID,
		Version:      "dev",
		TunnelID:     uuid.NewString(),
		Protocol:     "tcp",
		ExternalPort: externalPort,
		LocalHost:    c.localHost,
		LocalPort:    c.localPort,
	}); err != nil {
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

	errCh := make(chan error, 1)
	go func() {
		for {
			stream, err := session.AcceptStream()
			if err != nil {
				errCh <- err
				return
			}
			handler := c.streamHandlers[strings.ToLower(strings.TrimSpace("tcp"))]
			if handler == nil {
				handler = c.HandleTCPStream
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
