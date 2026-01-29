package relayserver

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/AidyyJ/PortOpener/internal/relay"
	"github.com/AidyyJ/PortOpener/server/internal/storage"
	"github.com/AidyyJ/PortOpener/server/internal/tunnels"
	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

type Config struct {
	Token string
}

type Server struct {
	token string
	reg   *tunnels.Registry
	store *storage.Store
	tcp   *TCPProxy
	udp   *UDPProxy
}

func New(cfg Config, registry *tunnels.Registry, store *storage.Store) *Server {
	return &Server{token: strings.TrimSpace(cfg.Token), reg: registry, store: store, tcp: &TCPProxy{Registry: registry, Store: store}, udp: &UDPProxy{Registry: registry, Store: store}}
}

func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var storedAuth bool
		if s.store != nil {
			ok, err := s.store.HasActiveToken()
			if err != nil {
				http.Error(w, "relay token lookup failed", http.StatusInternalServerError)
				return
			}
			storedAuth = ok
		}
		if !storedAuth && s.token == "" {
			http.Error(w, "relay token not configured", http.StatusServiceUnavailable)
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			log.Printf("relay accept failed: %v", err)
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		wsConn := websocket.NetConn(ctx, conn, websocket.MessageBinary)
		session, err := yamux.Server(wsConn, nil)
		if err != nil {
			log.Printf("relay yamux server failed: %v", err)
			_ = conn.Close(websocket.StatusInternalError, "yamux init failed")
			return
		}
		defer session.Close()

		control, err := session.AcceptStream()
		if err != nil {
			log.Printf("relay control stream failed: %v", err)
			return
		}
		defer control.Close()

		var hello relay.ControlMessage
		if err := relay.ReadJSON(control, &hello); err != nil {
			log.Printf("relay read hello failed: %v", err)
			return
		}
		defer func() {
			if s.reg != nil && hello.Subdomain != "" {
				s.reg.RemoveHTTP(hello.Subdomain)
			}
			if s.reg != nil && hello.Protocol == "tcp" && hello.ExternalPort != 0 {
				s.reg.RemoveTCP(hello.ExternalPort)
				if s.tcp != nil {
					s.tcp.RemoveListener(hello.ExternalPort)
				}
			}
			if s.reg != nil && hello.Protocol == "udp" && hello.ExternalPort != 0 {
				s.reg.RemoveUDP(hello.ExternalPort)
				if s.udp != nil {
					s.udp.RemoveListener(hello.ExternalPort)
				}
			}
		}()

		if hello.Type != "hello" {
			_ = relay.WriteJSON(control, relay.ControlMessage{
				Type:      "error",
				ErrorCode: "unauthorized",
				Message:   "invalid token",
			})
			return
		}
		if storedAuth {
			ok, err := s.store.ValidateToken(hello.Token)
			if err != nil {
				_ = relay.WriteJSON(control, relay.ControlMessage{
					Type:      "error",
					ErrorCode: "unauthorized",
					Message:   "invalid token",
				})
				return
			}
			if !ok {
				_ = relay.WriteJSON(control, relay.ControlMessage{
					Type:      "error",
					ErrorCode: "unauthorized",
					Message:   "invalid token",
				})
				return
			}
		} else if hello.Token != s.token {
			_ = relay.WriteJSON(control, relay.ControlMessage{
				Type:      "error",
				ErrorCode: "unauthorized",
				Message:   "invalid token",
			})
			return
		}

		if err := relay.WriteJSON(control, relay.ControlMessage{Type: "hello_ok", ClientID: hello.ClientID}); err != nil {
			log.Printf("relay hello_ok write failed: %v", err)
			return
		}

		if s.reg != nil && hello.Subdomain != "" {
			if err := s.reg.RegisterHTTP(hello.TunnelID, session, tunnels.HTTPRegistration{
				Subdomain: hello.Subdomain,
				Allowlist: hello.Allowlist,
			}); err != nil {
				_ = relay.WriteJSON(control, relay.ControlMessage{Type: "error", ErrorCode: "registration_failed", Message: err.Error()})
				return
			}
			if s.store != nil {
				if err := s.store.UpsertHTTPReservation(storage.HTTPReservation{
					TunnelID:  hello.TunnelID,
					Subdomain: hello.Subdomain,
					Allowlist: hello.Allowlist,
				}); err != nil {
					log.Printf("persist reservation failed: %v", err)
				}
				if err := s.store.UpsertTunnel(storage.Tunnel{
					ID:        hello.TunnelID,
					Protocol:  "http",
					LocalHost: hello.LocalHost,
					LocalPort: hello.LocalPort,
					Status:    "active",
					LastSeen:  time.Now().UTC(),
				}); err != nil {
					log.Printf("persist tunnel failed: %v", err)
				}
			}
		}

		if s.reg != nil && hello.Protocol == "tcp" && hello.ExternalPort != 0 {
			if err := s.reg.RegisterTCP(hello.TunnelID, session, hello.ExternalPort); err != nil {
				_ = relay.WriteJSON(control, relay.ControlMessage{Type: "error", ErrorCode: "registration_failed", Message: err.Error()})
				return
			}
			if s.store != nil {
				if err := s.store.UpsertPortReservation(storage.PortReservation{
					Protocol:     "tcp",
					ExternalPort: hello.ExternalPort,
					TunnelID:     hello.TunnelID,
					Reserved:     true,
				}); err != nil {
					log.Printf("persist port reservation failed: %v", err)
				}
				if err := s.store.UpsertTunnel(storage.Tunnel{
					ID:        hello.TunnelID,
					Protocol:  "tcp",
					LocalHost: hello.LocalHost,
					LocalPort: hello.LocalPort,
					Status:    "active",
					LastSeen:  time.Now().UTC(),
				}); err != nil {
					log.Printf("persist tunnel failed: %v", err)
				}
			}
			if s.tcp != nil {
				if err := s.tcp.EnsureListener(hello.ExternalPort); err != nil {
					log.Printf("tcp listener failed: %v", err)
				}
			}
		}

		if s.reg != nil && hello.Protocol == "udp" && hello.ExternalPort != 0 {
			if err := s.reg.RegisterUDP(hello.TunnelID, session, hello.ExternalPort); err != nil {
				_ = relay.WriteJSON(control, relay.ControlMessage{Type: "error", ErrorCode: "registration_failed", Message: err.Error()})
				return
			}
			if s.store != nil {
				if err := s.store.UpsertPortReservation(storage.PortReservation{
					Protocol:     "udp",
					ExternalPort: hello.ExternalPort,
					TunnelID:     hello.TunnelID,
					Reserved:     true,
				}); err != nil {
					log.Printf("persist port reservation failed: %v", err)
				}
				if err := s.store.UpsertTunnel(storage.Tunnel{
					ID:        hello.TunnelID,
					Protocol:  "udp",
					LocalHost: hello.LocalHost,
					LocalPort: hello.LocalPort,
					Status:    "active",
					LastSeen:  time.Now().UTC(),
				}); err != nil {
					log.Printf("persist tunnel failed: %v", err)
				}
			}
			if s.udp != nil {
				if err := s.udp.EnsureListener(hello.ExternalPort); err != nil {
					log.Printf("udp listener failed: %v", err)
				}
			}
		}

		go func() {
			for {
				stream, err := session.AcceptStream()
				if err != nil {
					return
				}
				_ = stream.Close()
			}
		}()

		lastHeartbeat := time.Now()
		for {
			var msg relay.ControlMessage
			if err := relay.ReadJSON(control, &msg); err != nil {
				log.Printf("relay control read failed: %v", err)
				return
			}
			if msg.Type == "heartbeat" {
				lastHeartbeat = time.Now()
				log.Printf("relay heartbeat from %s", hello.ClientID)
			} else if msg.Type != "" {
				log.Printf("relay message type=%s", msg.Type)
			}

			if time.Since(lastHeartbeat) > 30*time.Second {
				log.Printf("relay heartbeat timeout for %s", hello.ClientID)
				return
			}
		}
	}

}
