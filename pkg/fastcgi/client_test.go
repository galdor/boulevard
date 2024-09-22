package fastcgi

import (
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

	names := []string{
		"FCGI_MPXS_CONNS", // the only value returned by FPM
		"UNKNOWN_NAME",
	}

	pairs, err := c.FetchValues(names)
	require.NoError(err)

	require.Len(pairs, 1)
	require.Equal(names[0], pairs[0].Name)
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
