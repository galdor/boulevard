package fastcgi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.n16f.net/log"
)

const testServerAddress = "localhost:9000" // FPM running in Docker

func TestClientConnection(t *testing.T) {
	c := newTestClient(t)
	c.Close()
}

func TestClientValues(t *testing.T) {
	require := require.New(t)

	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	values, err := c.FetchValues(ctx)
	require.NoError(err)

	require.Len(values, 1) // FPM only sets FCGI_MPXS_CONNS
	require.Equal("FCGI_MPXS_CONNS", values[0].Name)
}

func newTestClient(t *testing.T) *Client {
	cfg := ClientCfg{
		Log:     log.DefaultLogger("fastcgi"),
		Address: testServerAddress,
	}

	c, err := NewClient(&cfg)
	if err != nil {
		t.Fatalf("cannot create client: %v", err)
	}

	return c
}
