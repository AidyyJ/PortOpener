package httpbridge

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/AidyyJ/PortOpener/internal/relay"
)

var ErrUnsupportedUpgrade = errors.New("websocket upgrade not supported yet")

func EncodeRequest(r *http.Request) (relay.HTTPRequest, []byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return relay.HTTPRequest{}, nil, err
	}
	_ = r.Body.Close()

	return relay.HTTPRequest{
		Method:      r.Method,
		Path:        r.URL.RequestURI(),
		Host:        r.Host,
		Header:      r.Header.Clone(),
		RemoteAddr:  r.RemoteAddr,
		IsWebSocket: isWebSocketRequest(r),
	}, body, nil
}

func DecodeRequest(req relay.HTTPRequest, body []byte) (*http.Request, error) {
	if req.IsWebSocket {
		return nil, ErrUnsupportedUpgrade
	}

	reader := bytes.NewReader(body)
	request, err := http.NewRequest(req.Method, req.Path, reader)
	if err != nil {
		return nil, err
	}
	request.Host = req.Host
	request.Header = req.Header.Clone()
	return request, nil
}

func EncodeResponse(resp *http.Response) (relay.HTTPResponse, []byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return relay.HTTPResponse{}, nil, err
	}
	_ = resp.Body.Close()
	return relay.HTTPResponse{Status: resp.StatusCode, Header: resp.Header.Clone()}, body, nil
}

func DecodeResponse(resp relay.HTTPResponse, body []byte) *http.Response {
	return &http.Response{
		StatusCode: resp.Status,
		Header:     resp.Header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func isWebSocketRequest(r *http.Request) bool {
	connection := strings.ToLower(r.Header.Get("Connection"))
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	return strings.Contains(connection, "upgrade") && upgrade == "websocket"
}
