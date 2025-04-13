package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPServeAction(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var res *http.Response
	var resBody string

	sendRequest := func(method, uriPath string) *http.Response {
		return c.SendRequest(method, uriPath, nil, nil, &resBody)
	}

	sendRequestWithHost := func(method, uriPath string) *http.Response {
		header := make(http.Header)
		header.Set("Host", "boulevard-serve.localhost")
		return c.SendRequest(method, uriPath, header, nil, &resBody)
	}

	// Root directory, no index file
	res = sendRequest("GET", "/serve")
	require.Equal(403, res.StatusCode)

	res = sendRequest("GET", "/serve/")
	require.Equal(403, res.StatusCode)

	res = sendRequestWithHost("GET", "/")
	require.Equal(403, res.StatusCode)

	// Missing file
	res = sendRequest("GET", "/serve/unknown")
	require.Equal(404, res.StatusCode)

	res = sendRequestWithHost("GET", "/unknown")
	require.Equal(404, res.StatusCode)

	// File with no extension
	//
	// XXX Right now there is a Content-Type header field because we rely on
	// http.ServeContent. To be fixed soon.
	res = sendRequest("GET", "/serve/a")
	require.Equal(200, res.StatusCode)
	//require.Equal(nil, res.Header.Values("Content-Type"))
	require.Equal("a", resBody)

	res = sendRequestWithHost("GET", "/a")
	require.Equal(200, res.StatusCode)
	require.Equal("a", resBody)

	// File with an extension
	res = sendRequest("GET", "/serve/b.json")
	require.Equal(200, res.StatusCode)
	require.Equal("application/json", res.Header.Get("Content-Type"))
	require.Equal("\"b\"", resBody)

	// File in subdirectory
	res = sendRequest("GET", "/serve/c/ca.txt")
	require.Equal(200, res.StatusCode)
	// XXX Right now the Content-Type is "text/plain; charset=utf-8" because
	// http.ServeContent stupidely analyzes the content. To be fixed soon.
	// require.Equal("text/plain", res.Header.Get("Content-Type"))
	require.Equal("ca", resBody)

	res = sendRequestWithHost("GET", "/c/ca.txt")
	require.Equal(200, res.StatusCode)
	require.Equal("ca", resBody)

	// Directory without an index file
	res = sendRequest("GET", "/serve/c")
	require.Equal(403, res.StatusCode)

	res = sendRequestWithHost("GET", "/c")
	require.Equal(403, res.StatusCode)

	// Directory with an index file
	res = sendRequest("GET", "/serve/d")
	require.Equal(200, res.StatusCode)
	require.Equal("d", resBody)

	res = sendRequestWithHost("GET", "/serve/d")
	require.Equal(200, res.StatusCode)
	require.Equal("d", resBody)
}
