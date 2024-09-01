package service

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.n16f.net/boulevard/pkg/httputils"
)

func TestServiceBasicAuth(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var uriPath string
	var res *http.Response

	sendRequest := func(uriPath string, headerFields ...string) *http.Response {
		header := httputils.Header(headerFields...)
		return c.SendRequest("GET", uriPath, header, nil, nil)
	}

	basicAuthorization := func(username, password string) string {
		credentials := []byte(username + ":" + password)
		return "Basic " + base64.StdEncoding.EncodeToString(credentials)
	}

	uriPath = "/auth/basic/credentials"

	// No authorization field
	res = sendRequest(uriPath)
	require.Equal(401, res.StatusCode)

	// Empty authorization field
	res = sendRequest(uriPath, "Authorization", "")
	require.Equal(401, res.StatusCode)

	// Invalid authorization format
	res = sendRequest(uriPath, "Authorization", "foobar")
	require.Equal(401, res.StatusCode)

	// Invalid authorization scheme
	res = sendRequest(uriPath, "Authorization", "Bearer bar")
	require.Equal(401, res.StatusCode)

	// Invalid credentials format
	res = sendRequest(uriPath, "Authorization", "Basic foo")
	require.Equal(403, res.StatusCode)

	// Invalid credentials
	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("eve", "foo"))
	require.Equal(403, res.StatusCode)

	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("bob", "hello"))
	require.Equal(403, res.StatusCode)

	// Valid credentials
	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("bob", "foo"))
	require.Equal(200, res.StatusCode)

	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("alice", "bar"))
	require.Equal(200, res.StatusCode)

	// Credential files
	uriPath = "/auth/basic/credential-file"

	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("eve", "foo"))
	require.Equal(403, res.StatusCode)

	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("bob", "foo"))
	require.Equal(200, res.StatusCode)
}

func TestServiceBearerAuth(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var uriPath string
	var res *http.Response

	sendRequest := func(uriPath string, headerFields ...string) *http.Response {
		header := httputils.Header(headerFields...)
		return c.SendRequest("GET", uriPath, header, nil, nil)
	}

	basicAuthorization := func(token string) string {
		return "Bearer " + token
	}

	uriPath = "/auth/bearer/tokens"

	// No authorization field
	res = sendRequest(uriPath)
	require.Equal(401, res.StatusCode)

	// Empty authorization field
	res = sendRequest(uriPath, "Authorization", "")
	require.Equal(401, res.StatusCode)

	// Invalid authorization format
	res = sendRequest(uriPath, "Authorization", "foobar")
	require.Equal(401, res.StatusCode)

	// Invalid authorization scheme
	res = sendRequest(uriPath, "Authorization", "Basic bar")
	require.Equal(401, res.StatusCode)

	// Invalid credentials
	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("hello"))
	require.Equal(403, res.StatusCode)

	// Valid credentials
	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("foo"))
	require.Equal(200, res.StatusCode)

	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("bar"))
	require.Equal(200, res.StatusCode)

	// Credential files
	uriPath = "/auth/bearer/token-file"

	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("hello"))
	require.Equal(403, res.StatusCode)

	res = sendRequest(uriPath, "Authorization",
		basicAuthorization("foo"))
	require.Equal(200, res.StatusCode)
}
