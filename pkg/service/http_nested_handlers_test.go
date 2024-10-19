package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.n16f.net/boulevard/pkg/httputils"
)

func TestHTTPNestedHandlers(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var res *http.Response
	var resBody string

	sendRequest := func(uriPath string, bearerToken string) *http.Response {
		var header http.Header
		if bearerToken != "" {
			header = httputils.Header("Authorization", "Bearer "+bearerToken)
		}

		return c.SendRequest("GET", uriPath, header, nil, &resBody)
	}

	// Top-level matches but no subhandler matches
	res = sendRequest("/nested", "")
	require.Equal(200, res.StatusCode)
	require.Equal("default", resBody)

	// Second level does not match
	res = sendRequest("/nested/baz", "")
	require.Equal(200, res.StatusCode)
	require.Equal("default", resBody)

	// Second level with no subhandler matches
	res = sendRequest("/nested/foo", "")
	require.Equal(200, res.StatusCode)
	require.Equal("foo", resBody)

	// Second level with subhandlers matches but no subhandler matches
	res = sendRequest("/nested/bar", "")
	require.Equal(401, res.StatusCode)

	res = sendRequest("/nested/bar", "foo")
	require.Equal(200, res.StatusCode)
	require.Equal("bar", resBody)

	// Third level matches
	res = sendRequest("/nested/bar/y", "")
	require.Equal(401, res.StatusCode)

	res = sendRequest("/nested/bar/y", "foo")
	require.Equal(200, res.StatusCode)
	require.Equal("y", resBody)
}
