package service

import (
	"net/http"
	"strings"
	"testing"

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
