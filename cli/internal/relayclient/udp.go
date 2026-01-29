package relayclient

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"time"

	"github.com/AidyyJ/PortOpener/internal/relay"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

func (c *Client) RegisterUDP(ctx context.Context, externalPort int) error {
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
		Protocol:     "udp",
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
			go c.HandleUDPStream(ctx, stream)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Client) HandleUDPStream(ctx context.Context, stream *yamux.Stream) {
	defer stream.Close()
	var msg relay.UDPDatagram
	if err := relay.ReadJSON(stream, &msg); err != nil {
		return
	}
	data, err := base64.StdEncoding.DecodeString(msg.PayloadB64)
	if err != nil {
		return
	}
	localHost := c.localHost
	if localHost == "" {
		localHost = "127.0.0.1"
	}
	addr := &net.UDPAddr{IP: net.ParseIP(localHost), Port: c.localPort}
	if addr.IP == nil {
		addr.IP = net.IPv4(127, 0, 0, 1)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	defer conn.Close()

	_, _ = conn.Write(data)
	buf := make([]byte, 65535)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return
	}
	resp := relay.UDPDatagram{RemoteAddr: msg.RemoteAddr, PayloadB64: base64.StdEncoding.EncodeToString(buf[:n])}
	_ = relay.WriteJSON(stream, resp)
}
