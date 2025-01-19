package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPReplyAction(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var res *http.Response
	var resBody string

	sendRequest := func(uriPath string) *http.Response {
		return c.SendRequest("GET", uriPath, nil, nil, &resBody)
	}

	res = sendRequest("/hello")
	require.Equal(200, res.StatusCode)
	require.Equal("Boulevard", res.Header.Get("Server"))
	require.Equal("world", resBody)
}
