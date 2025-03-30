package service

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"

	"go.n16f.net/boulevard/pkg/netutils"
)

type TestTCPServer struct {
	listener net.Listener

	t  *testing.T
	wg sync.WaitGroup
}

func NewTestTCPServer(t *testing.T, address string) *TestTCPServer {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("cannot listen on %q: %v", address, err)
	}

	s := TestTCPServer{
		t:        t,
		listener: listener,
	}

	s.wg.Add(1)
	go s.accept()

	return &s
}

func (s *TestTCPServer) Stop() {
	s.listener.Close()
	s.wg.Wait()
}

func (s *TestTCPServer) accept() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}

			s.t.Fatalf("cannot accept connection: %v", err)
		}

		s.t.Logf("accepted connection from %v", conn.RemoteAddr())

		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *TestTCPServer) handleConn(conn net.Conn) {
	defer s.wg.Done()

	buf := make([]byte, 4096)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return
			}

			err = netutils.UnwrapOpError(err, "read")
			s.t.Fatalf("cannot read connection: %v", err)
		}

		if _, err := conn.Write(buf[:n]); err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}

			err = netutils.UnwrapOpError(err, "write")
			s.t.Fatalf("cannot write upstream connection: %v", err)
		}
	}
}
