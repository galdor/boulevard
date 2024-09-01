package httputils

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"go.n16f.net/program"
)

type TestClient struct {
	httpClient *http.Client
	baseURI    *url.URL

	t *testing.T
}

func NewTestClient(t *testing.T, baseURI *url.URL) *TestClient {
	c := TestClient{
		httpClient: http.DefaultClient,
		baseURI:    baseURI,

		t: t,
	}

	return &c
}

func (c *TestClient) SendRequest(method, uriRefString string, header http.Header, reqBody io.Reader, resBody *[]byte) *http.Response {
	uriRef, err := url.Parse(uriRefString)
	if err != nil {
		c.fail("cannot parse URI reference %q: %v", uriRefString, err)
	}

	uri := c.baseURI.ResolveReference(uriRef)

	req, err := http.NewRequest(method, uri.String(), reqBody)
	if err != nil {
		c.fail("cannot create HTTP request: %v", err)
	}

	for name, values := range header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		c.fail("cannot send request: %v", err)
	}
	defer res.Body.Close()

	resBodyData, err := io.ReadAll(res.Body)
	if err != nil {
		c.fail("cannot read response body: %v", err)
	}

	if resBody != nil {
		*resBody = resBodyData
	}

	return res
}

func (c *TestClient) fail(format string, args ...any) {
	c.t.Error(program.StackTrace(0, 20, true))
	c.t.Fatalf(format, args...)
}
