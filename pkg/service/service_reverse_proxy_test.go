package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceReverseProxy(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var res *http.Response
	var resBody string

	// GET request without a body
	res = c.SendRequest("GET", "/backend/ping", nil, nil, &resBody)
	require.Equal(200, res.StatusCode)
	require.Equal("pong", resBody)

	// POST request with a body
	res = c.SendRequest("POST", "/backend/data", nil, "foo", &resBody)
	require.Equal(200, res.StatusCode)
	require.Equal("ok", resBody) // TODO "3" (requires templating)
}
