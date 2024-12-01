package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

type TestWSServer struct {
	httpServer *http.Server

	t *testing.T
}

func NewTestWSServer(t *testing.T, address string) *TestWSServer {
	s := TestWSServer{
		t: t,
	}

	s.httpServer = &http.Server{
		Addr:    address,
		Handler: &s,
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("cannot listen on %q: %v", address, err)
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil {
			if err != http.ErrServerClosed {
				t.Errorf("cannot serve: %v", err)
			}
		}
	}()

	return &s
}

func (s *TestWSServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	s.httpServer.Shutdown(ctx)
}

func (s *TestWSServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var upgrader websocket.Upgrader
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		s.t.Fatalf("WebSocket upgrade error: %v", err)
	}
	defer conn.Close()

	checkErr := func(err error, format string, args ...any) bool {
		if err != nil {
			var closeErr *websocket.CloseError

			switch {
			case errors.As(err, &closeErr):
			default:
				s.t.Fatalf(format, args...)
			}

			return true
		}

		return false
	}

	for {
		_, r, err := conn.NextReader()
		if checkErr(err, "cannot obtain WebSocket reader: %v", err) {
			return
		}

		data, err := io.ReadAll(r)
		if checkErr(err, "cannot read WebSocket: %v", err) {
			return
		}

		w, err := conn.NextWriter(websocket.TextMessage)
		if checkErr(err, "cannot obtain WebSocket writer: %v", err) {
			return
		}

		msg := strconv.Itoa(len(data))
		_, err = io.Copy(w, strings.NewReader(msg))
		if checkErr(err, "cannot write WebSocket: %v", err) {
			return
		}

		err = w.Close()
		if checkErr(err, "cannot close WebSocket writer: %v", err) {
			return
		}
	}
}
