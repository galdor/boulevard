package service

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTCPReverseProxy(t *testing.T) {
	require := require.New(t)

	upstream := NewTestTCPServer(t, ":9011")
	defer upstream.Stop()

	client, err := net.Dial("tcp", "localhost:9010")
	require.NoError(err)
	defer client.Close()

	_, err = client.Write([]byte("hello"))
	require.NoError(err)

	var buf [1024]byte
	n, err := client.Read(buf[:])
	require.NoError(err)
	require.Equal(5, n)
	require.Equal("hello", string(buf[:5]))
}
