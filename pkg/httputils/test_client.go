package httputils

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.n16f.net/program"
)

type TestClient struct {
	httpClient *http.Client
	baseURI    *url.URL

	t *testing.T
}

func NewTestClient(t *testing.T, baseURI *url.URL) *TestClient {
	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	transport := http.Transport{
		DialContext: dialer.DialContext,

		MaxIdleConns: 10,

		IdleConnTimeout:       10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := http.Client{
		Timeout:   10 * time.Second,
		Transport: &transport,

		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	c := TestClient{
		httpClient: &client,
		baseURI:    baseURI,

		t: t,
	}

	return &c
}

func (c *TestClient) SendRequest(method, uriRefString string, header http.Header, reqBody, resBody any) *http.Response {
	uriRef, err := url.Parse(uriRefString)
	if err != nil {
		c.fail("cannot parse URI reference %q: %v", uriRefString, err)
	}

	uri := c.baseURI.ResolveReference(uriRef)

	req, err := http.NewRequest(method, uri.String(), c.reqBodyReader(reqBody))
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

	if err := c.readResponseBody(res.Body, resBody); err != nil {
		c.fail("cannot read response body: %v", err)
	}

	return res
}

func (c *TestClient) reqBodyReader(reqBody any) io.Reader {
	var r io.Reader

	switch rb := reqBody.(type) {
	case nil:
		return nil
	case io.Reader:
		r = rb
	case []byte:
		r = bytes.NewReader(rb)
	case string:
		r = strings.NewReader(rb)
	default:
		program.Panic("unhandled request body %#v (%T)", reqBody, reqBody)
	}

	return r
}

func (c *TestClient) readResponseBody(r io.Reader, resBody any) error {
	switch rb := resBody.(type) {
	case nil:
		return nil
	case io.Writer:
		_, err := io.Copy(rb, r)
		return err
	case *[]byte:
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		*rb = data
	case *string:
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		*rb = string(data)
	default:
		program.Panic("unhandled response body %#v (%T)", resBody, resBody)
	}

	return nil
}

func (c *TestClient) fail(format string, args ...any) {
	c.t.Error(program.StackTrace(0, 20, true))
	c.t.Fatalf(format, args...)
}
