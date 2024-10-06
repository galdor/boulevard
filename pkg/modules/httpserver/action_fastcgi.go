package httpserver

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	"go.n16f.net/boulevard/pkg/fastcgi"
	"go.n16f.net/ejson"
)

type FastCGIActionCfg struct {
	Address      string            `json:"address"`
	Parameters   map[string]string `json:"parameters,omitempty"`
	Path         string            `json:"path,omitempty"`
	ScriptRegexp string            `json:"string_regexp,omitempty"`
}

func (cfg *FastCGIActionCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckNetworkAddress("address", cfg.Address)
}

type FastCGIAction struct {
	Handler *Handler
	Cfg     FastCGIActionCfg

	client *fastcgi.Client

	scriptRE *regexp.Regexp
}

func NewFastCGIAction(h *Handler, cfg FastCGIActionCfg) (*FastCGIAction, error) {
	a := FastCGIAction{
		Handler: h,
		Cfg:     cfg,
	}

	if s := cfg.ScriptRegexp; s != "" {
		if s[0] != '^' && s[0] != '/' {
			return nil, fmt.Errorf("script regexp must match an absolute path")
		}

		if s[0] != '^' {
			s = "^" + s
		}

		re, err := regexp.Compile(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse script regexp: %w", err)
		}

		a.scriptRE = re
	}

	return &a, nil
}

func (a *FastCGIAction) Start() error {
	clientCfg := fastcgi.ClientCfg{
		Log:     a.Handler.Module.Log,
		Address: a.Cfg.Address,
	}

	client, err := fastcgi.NewClient(&clientCfg)
	if err != nil {
		return fmt.Errorf("cannot create FastCGI client: %w", err)
	}

	a.client = client

	return nil
}

func (a *FastCGIAction) Stop() {
	a.client.Close()
}

func (a *FastCGIAction) HandleRequest(ctx *RequestContext) {
	// In theory we should be able to stream the request body, which would be
	// nice if the client used chunked transfer encoding. But if we do not
	// buffer the request body, we have no way to compute its length, meaning we
	// cannot set the mandatory CONTENT_LENGTH FastCGI parameter.
	reqBody, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		a.Handler.Module.Log.Error("cannot read request body: %v", err)
		ctx.ReplyError(500)
		return
	}

	params := a.requestParameters(ctx, reqBody)
	stdin := bytes.NewReader(reqBody)

	res, err := a.client.SendRequest(fastcgi.RoleResponder, params, stdin, nil)
	if err != nil {
		a.Handler.Module.Log.Error("cannot send FastCGI request: %v", err)

		status := 500

		if errors.Is(err, fastcgi.ErrClientShutdown) {
			status = 503
		} else if errors.Is(err, fastcgi.ErrServerOverloaded) {
			status = 503
		} else if errors.Is(err, fastcgi.ErrTooManyConcurrentRequests) {
			status = 503
		}

		ctx.ReplyError(status)
		return
	}

	// We have to buffer the response body since we need to send a
	// Content-Length header field. We could use chunked transfer encoding if
	// the client supports HTTP 1.1, but we would still have to implement
	// buffering for HTTP 1.0 clients.
	var resBody bytes.Buffer

	for event := range res.Events {
		if event == nil {
			break
		}

		if event.Error != nil {
			a.Handler.Module.Log.Error("cannot read FastCGI response: %v",
				event.Error)
			ctx.ReplyError(502)
			return
		}

		resBody.Write(event.Data)
	}

	header := ctx.ResponseWriter.Header()
	res.CopyHeaderToHTTPHeader(header)

	header.Set("Content-Length", strconv.Itoa(resBody.Len()))

	// It would be nice to be able to use the reason string, but the
	// http.ResponseWriter interface does not support it.
	statusCode, _ := res.Status()
	ctx.Reply(statusCode, &resBody)
}

func (a *FastCGIAction) requestParameters(ctx *RequestContext, reqBody []byte) fastcgi.NameValuePairs {
	req := ctx.Request
	header := req.Header

	var pathInfo string
	var scriptName string

	absSubpath := "/" + ctx.Subpath
	if a.scriptRE == nil {
		scriptName = absSubpath
		pathInfo = "/"
	} else {
		if match := a.scriptRE.FindString(absSubpath); match == "" {
			scriptName = absSubpath
			pathInfo = "/"
		} else {
			scriptName = match
			pathInfo = strings.TrimPrefix(absSubpath, match)
		}
	}

	serverPort := ctx.Listener.TCPListener.Port

	buildId := a.Handler.Module.Data.BoulevardBuildId

	basePath := a.Cfg.Path
	if basePath == "" {
		basePath = "/"
	}

	params := map[string]string{
		// RFC 3875 4.1. Request Meta-Variables
		"CONTENT_LENGTH":    strconv.Itoa(len(reqBody)),
		"CONTENT_TYPE":      header.Get("Content-Type"),
		"GATEWAY_INTERFACE": "CGI/1.1",
		"PATH_INFO":         pathInfo,
		"PATH_TRANSLATED":   path.Join(basePath, pathInfo),
		"QUERY_STRING":      req.URL.RawQuery,
		"REMOTE_ADDR":       ctx.ClientAddress.String(),
		"REMOTE_HOST":       ctx.ClientAddress.String(),  // [1]
		"REQUEST_METHOD":    strings.ToUpper(req.Method), // [2]
		"SCRIPT_NAME":       scriptName,
		"SERVER_NAME":       ctx.Host,
		"SERVER_PORT":       strconv.Itoa(serverPort),
		"SERVER_PROTOCOL":   req.Proto,
		"SERVER_SOFTWARE":   "boulevard/" + buildId,

		// Required for php-fpm
		"SCRIPT_FILENAME": path.Join(basePath, scriptName),
	}

	// [1] 4.1.9. REMOTE_HOST: "If the hostname is not available for performance
	// reasons or otherwise, the server MAY substitute the REMOTE_ADDR value".
	// Because no, we are not going to do a reverse DNS lookup for each request.
	//
	// [2] 4.1.12. REQUEST_METHOD: "The method is case sensitive".

	if scheme, _, ok := strings.Cut(header.Get("Authorization"), " "); ok {
		params["AUTH_TYPE"] = scheme
	}

	if username, _, ok := req.BasicAuth(); ok {
		params["REMOTE_USER"] = username
	}

	// RFC 3875 4.1.18. Protocol-Specific Meta-Variables
	for name, values := range header {
		name = "HTTP_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		value := strings.Join(values, ", ")

		params[name] = value
	}

	for name, value := range a.Cfg.Parameters {
		params[name] = value
	}

	pairs := make(fastcgi.NameValuePairs, len(params))

	i := 0
	for name, value := range params {
		pairs[i].Name = name
		pairs[i].Value = value
		i++
	}

	return pairs
}
