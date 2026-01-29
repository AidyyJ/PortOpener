# Relay Transport & Framing (Phase 1)

Authoritative scope: [`docs/chat.md`](chat.md:1)

This document fixes the Phase 1 decision for the data-plane transport and
multiplexing strategy, and defines the initial framing for HTTP/TCP/UDP relay.

## Transport choice

- **Base transport**: WebSocket over HTTPS (via Caddy).
- **Go WebSocket library**: `github.com/coder/websocket` (nhooyr) to provide
  a `net.Conn`-compatible stream via `websocket.NetConn`.
- **Multiplexer**: `github.com/hashicorp/yamux` for logical streams over the
  WebSocket connection.

Rationale:

- WebSocket works through the Caddy edge and is compatible with HTTPS-only
  ingress.
- `websocket.NetConn` lets us reuse Go net/http and stream interfaces.
- yamux provides lightweight stream multiplexing for many tunnels and
  concurrent proxied connections.

## Session model

Each CLI process establishes a single **relay session** to the server:

1. CLI dials `wss://<base>/relay` through Caddy.
2. Server authenticates the CLI token and upgrades to WebSocket.
3. The WebSocket is wrapped into a `net.Conn` and used as the **yamux base
   connection**.
4. Logical streams are opened as needed.

Stream types:

- `control` stream: one per session for registration, heartbeats, and
  administrative messages (JSON frames).
- `http` stream: one per HTTP request (including WebSocket upgrades).
- `tcp` stream: one per inbound TCP connection.
- `udp` stream: one per UDP tunnel, carrying framed datagrams.

## Framing conventions

All structured frames use length-prefix framing:

- 4-byte big-endian `uint32` length
- followed by that many bytes of payload (JSON or binary)

This avoids partial read ambiguity and keeps framing simple.

### Control stream (JSON frames)

Payloads are JSON objects encoded as UTF-8, length-prefixed.

Example message shapes:

```json
{"type":"hello","client_id":"<uuid>","token":"<redacted>","version":"dev"}
```

```json
{"type":"heartbeat","timestamp":"2026-01-01T00:00:00Z"}
```

```json
{"type":"register_tunnel","tunnel_id":"<uuid>","protocol":"http","subdomain":"app"}
```

```json
{"type":"register_ok","tunnel_id":"<uuid>"}
```

Errors use:

```json
{"type":"error","code":"unauthorized","message":"invalid token"}
```

### HTTP stream

The HTTP stream carries two segments:

1. **Request header frame**: JSON envelope with metadata (method, path, host,
   headers, remote_addr, is_websocket).
2. **Request body frames**: zero or more length-prefixed binary frames
   (raw bytes).

After the request body is fully sent, the CLI replies on the same stream with:

1. **Response header frame**: JSON with status + headers.
2. **Response body frames**: length-prefixed binary frames.

If `is_websocket=true`, the stream becomes a bidirectional byte pipe *after*
the response header frame is written. The server then proxies frames directly
between the client connection and the stream.

### TCP stream

The TCP stream is a raw byte pipe. The server opens a stream per accepted
connection and the CLI dials the local target. Bytes are copied in both
directions until either side closes.

### UDP stream

UDP uses framed datagrams on a long-lived stream (one per UDP tunnel).

Datagram frame:

```json
{
  "remote_addr": "203.0.113.10:54321",
  "payload_b64": "..."
}
```

The JSON envelope is length-prefixed. This format allows the server to track
`{tunnel_id, remote_addr}` session mappings and enforce idle timeouts.

## Heartbeats and timeouts

- CLI sends heartbeat frames on the control stream every 10 seconds.
- Server considers a relay session stale after 30 seconds without heartbeat.
- Tunnel entries are cleaned up on session drop.

## Next steps

- Implement `/relay` WebSocket endpoint in the server.
- Implement CLI relay dial + control stream handshake.
- Add yamux stream routing for HTTP/TCP/UDP.

