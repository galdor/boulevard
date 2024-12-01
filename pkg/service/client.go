package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.n16f.net/ejson"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(path string) *Client {
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		return net.Dial("unix", path)
	}

	transport := http.Transport{
		DialContext: dial,
	}

	httpClient := http.Client{
		Timeout:   30 * time.Second,
		Transport: &transport,
	}

	c := Client{
		httpClient: &httpClient,
	}

	return &c
}

func (c *Client) Call(op string, reqBody, resBody any) (*http.Response, error) {
	uri := url.URL{
		Scheme: "http",
		Host:   "localhost",
		Path:   "/" + op,
	}

	var reqBodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("cannot encode request body: %w", err)
		}

		reqBodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", uri.String(), reqBodyReader)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot send request: %w", err)
	}
	defer res.Body.Close()

	resBodyData, err := io.ReadAll(res.Body)
	if err != nil {
		return res, fmt.Errorf("cannot read response body: %w", err)
	}

	if status := res.StatusCode; status < 200 || status > 399 {
		var baseError error

		var apiError ControlAPIError
		if err := json.Unmarshal(resBodyData, &apiError); err == nil {
			baseError = &apiError
		} else {
			baseError = errors.New(string(resBodyData))
		}

		return res, fmt.Errorf("request failed with status %d: %w",
			res.StatusCode, baseError)
	}

	if resBody != nil {
		if err := ejson.Unmarshal(resBodyData, resBody); err != nil {
			return res, fmt.Errorf("cannot decode response body: %w", err)
		}
	}

	return res, nil
}
