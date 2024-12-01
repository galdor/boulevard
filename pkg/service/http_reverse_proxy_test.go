package service

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestHTTPReverseProxy(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var res *http.Response
	var resBody string

	// GET request without a body
	res = c.SendRequest("GET", "/nginx/ping", nil, nil, &resBody)
	require.Equal(200, res.StatusCode)
	require.Equal("pong", resBody)

	// POST request with a body
	res = c.SendRequest("POST", "/nginx/data", nil, "foo", &resBody)
	require.Equal(200, res.StatusCode)
	require.Equal("3", resBody)

	// Redirection
	res = c.SendRequest("GET", "/nginx/redirect/foo", nil, nil, nil)
	require.Equal(302, res.StatusCode)
	require.True(strings.HasSuffix(res.Header.Get("Location"), "/hello/foo"))
}

func TestHTTPReverseProxyWebSocket(t *testing.T) {
	require := require.New(t)

	wsServer := NewTestWSServer(t, "localhost:9003")
	defer wsServer.Stop()

	uri := "ws://localhost:8080/websocket"
	conn, res, err := websocket.DefaultDialer.Dial(uri, nil)
	require.Equal(101, res.StatusCode)
	require.NoError(err)
	defer conn.Close()

	roundtrip := func(msg string) (string, error) {
		w, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return "", fmt.Errorf("cannot obtain WebSocket writer: %w", err)
		}

		if _, err := w.Write([]byte(msg)); err != nil {
			return "", fmt.Errorf("cannot write WebSocket: %w", err)
		}

		if err := w.Close(); err != nil {
			return "", fmt.Errorf("cannot close WebSocket writer: %w", err)
		}

		_, r, err := conn.NextReader()
		if err != nil {
			return "", fmt.Errorf("cannot obtain WebSocket reader: %w", err)
		}

		data, err := io.ReadAll(r)
		if err != nil {
			return "", fmt.Errorf("cannot read WebSocket: %v", err)
		}

		return string(data), nil
	}

	msg1, err := roundtrip("foobar")
	require.NoError(err)
	require.Equal("6", msg1)

	msg2, err := roundtrip("")
	require.NoError(err)
	require.Equal("0", msg2)

	msg3, err := roundtrip("hello world!")
	require.NoError(err)
	require.Equal("12", msg3)
}
