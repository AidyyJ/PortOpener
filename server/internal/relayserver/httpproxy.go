package relayserver

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/AidyyJ/PortOpener/internal/httpbridge"
	"github.com/AidyyJ/PortOpener/internal/relay"
	"github.com/AidyyJ/PortOpener/server/internal/metrics"
	"github.com/AidyyJ/PortOpener/server/internal/storage"
	"github.com/AidyyJ/PortOpener/server/internal/tunnels"
	"github.com/coder/websocket"
)

type HTTPProxy struct {
	Registry *tunnels.Registry
	Metrics  *metrics.Collector
	Logs     *metrics.Logger
	Store    *storage.Store
}

func (p *HTTPProxy) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if p.Registry == nil {
			http.Error(w, "registry not configured", http.StatusServiceUnavailable)
			return
		}

		if strings.HasPrefix(strings.ToLower(r.Host), "admin.") {
			http.NotFound(w, r)
			return
		}

		host := strings.ToLower(r.Host)
		if strings.Contains(host, ":") {
			if parsedHost, _, err := net.SplitHostPort(host); err == nil {
				host = parsedHost
			}
		}
		parts := strings.Split(host, ".")
		if len(parts) == 0 {
			http.NotFound(w, r)
			return
		}
		subdomain := parts[0]
		entry, ok := p.Registry.LookupHTTP(subdomain)
		if !ok {
			// Attempt custom domain routing via storage mapping
			if p.Store == nil {
				http.NotFound(w, r)
				return
			}
			mapped, found, err := p.Store.GetCustomDomain(host)
			if err != nil || !found {
				http.NotFound(w, r)
				return
			}
			if strings.ToLower(mapped.Status) != "enabled" {
				http.NotFound(w, r)
				return
			}
			entry, ok = p.Registry.LookupHTTPByTunnelID(mapped.TunnelID)
			if !ok {
				http.NotFound(w, r)
				return
			}
		}

		allow, err := tunnels.ParseAllowlist(entry.Allowlist)
		if err != nil || !allow.Allows(r.RemoteAddr) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if entry.Session == nil {
			http.Error(w, "tunnel unavailable", http.StatusServiceUnavailable)
			return
		}

		stream, err := entry.Session.OpenStream()
		if err != nil {
			http.Error(w, "relay unavailable", http.StatusBadGateway)
			return
		}
		defer stream.Close()

		reqFrame, body, err := httpbridge.EncodeRequest(r)
		if err != nil {
			http.Error(w, "encode request failed", http.StatusBadRequest)
			return
		}

		if err := relay.WriteJSON(stream, reqFrame); err != nil {
			log.Printf("relay write request failed: %v", err)
			http.Error(w, "relay failed", http.StatusBadGateway)
			return
		}
		if !reqFrame.IsWebSocket {
			if err := relay.WriteFrame(stream, body); err != nil {
				log.Printf("relay write body failed: %v", err)
				http.Error(w, "relay failed", http.StatusBadGateway)
				return
			}
		}

		var respFrame relay.HTTPResponse
		if err := relay.ReadJSON(stream, &respFrame); err != nil {
			log.Printf("relay read response failed: %v", err)
			http.Error(w, "relay failed", http.StatusBadGateway)
			return
		}
		if reqFrame.IsWebSocket {
			if err := proxyWebSocket(w, r, respFrame, stream); err != nil {
				log.Printf("websocket proxy failed: %v", err)
			}
			return
		}

		respBody, err := relay.ReadFrame(stream)
		if err != nil {
			log.Printf("relay read body failed: %v", err)
			http.Error(w, "relay failed", http.StatusBadGateway)
			return
		}

		resp := httpbridge.DecodeResponse(respFrame, respBody)
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)

		bytesIn := int64(len(body))
		bytesOut := int64(len(respBody))
		if p.Metrics != nil {
			p.Metrics.Add(entry.TunnelID, 1, bytesIn, bytesOut)
		}
		if p.Logs != nil {
			p.Logs.Add(metrics.LogEntry{
				TunnelID:   entry.TunnelID,
				Timestamp:  time.Now().UTC(),
				RemoteAddr: r.RemoteAddr,
				Method:     r.Method,
				Path:       r.URL.Path,
				Status:     resp.StatusCode,
				BytesIn:    bytesIn,
				BytesOut:   bytesOut,
			})
		}
		if p.Store != nil {
			_ = p.Store.InsertLog(storage.LogEntry{
				TunnelID:   entry.TunnelID,
				Timestamp:  time.Now().UTC(),
				Kind:       "http",
				RemoteAddr: r.RemoteAddr,
				Summary:    r.Method + " " + r.URL.Path,
				Status:     resp.StatusCode,
				BytesIn:    bytesIn,
				BytesOut:   bytesOut,
			})
			_ = p.Store.AddMetric(entry.TunnelID, time.Now().UTC(), 1, 0, bytesIn, bytesOut)
		}
	}
}

func proxyWebSocket(w http.ResponseWriter, r *http.Request, resp relay.HTTPResponse, stream io.ReadWriter) error {
	if resp.Status != http.StatusSwitchingProtocols {
		w.WriteHeader(resp.Status)
		return nil
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	ctx := r.Context()
	wsConn := websocket.NetConn(ctx, conn, websocket.MessageBinary)

	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(wsConn, relay.NewFrameReader(stream))
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(relay.NewFrameWriter(stream), wsConn)
		errCh <- err
	}()

	<-errCh
	return nil
}
