package service

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.n16f.net/acme/pkg/acme"
	"go.n16f.net/boulevard/pkg/httputils"
)

func TestACMEHTTPListener(t *testing.T) {
	require := require.New(t)

	baseURI := url.URL{
		Scheme: "http",
		Host:   acme.PebbleHTTPChallengeSolverAddress,
	}

	c := httputils.NewTestClient(t, &baseURI)

	var res *http.Response

	// Requests to an ACME validation URI are handled by the challenge solver
	res = c.SendRequest("GET", "/.well-known/acme-challenge/foo", nil, nil, nil)
	require.Equal(400, res.StatusCode) // unknown token

	// Requests to other URIs are forwarded to the upstream URI
	res = c.SendRequest("GET", "/hello", nil, nil, nil)
	require.Equal(200, res.StatusCode)
}
