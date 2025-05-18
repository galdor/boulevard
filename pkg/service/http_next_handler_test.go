package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPNextHandler(t *testing.T) {
	require := require.New(t)

	c := testHTTPClient(t)

	var res *http.Response
	var resBody string

	sendRequest := func(uriPath string) *http.Response {
		return c.SendRequest("GET", uriPath, nil, nil, &resBody)
	}

	// Matching handler after the one with next_handler. The authentication
	// configuration was propagated.
	res = sendRequest("/next-handler/a")
	require.Equal(401, res.StatusCode)

	// No matching handlers after the one with next_handler, the handler of
	// /next-handler/ is the last match.
	res = sendRequest("/next-handler/b")
	require.Equal(200, res.StatusCode)
	require.Equal(resBody, "default")
}
