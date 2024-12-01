package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestControlAPI(t *testing.T) {
	require := require.New(t)

	c := testControlAPIClient(t)

	var res *http.Response
	var err error

	res, err = c.Call("foo", nil, nil)
	require.Error(err)
	require.Equal(404, res.StatusCode)
}

func testControlAPIClient(t *testing.T) *Client {
	path := "/tmp/boulevard.sock"
	return NewClient(path)
}
