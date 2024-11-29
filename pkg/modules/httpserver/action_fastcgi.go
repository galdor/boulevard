package httpserver

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/fastcgi"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

const (
	DefaultRequestBodyMemoryBufferSize = 128 * 1024
	DefaultMaxRequestBodySize          = 4 * 1024 * 1024

	DefaultResponseBodyMemoryBufferSize = 128 * 1024
	DefaultMaxResponseBodySize          = 4 * 1024 * 1024

	DefaultRequestTimeout = 10.0 // seconds
)

type FastCGIActionCfg struct {
	Address       string            `json:"address"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	Path          string            `json:"path,omitempty"`
	DefaultScript string            `json:"default_script,omitempty"`
	ScriptRegexp  string            `json:"script_regexp,omitempty"`

	TemporaryDirectoryPath string `json:"temporary_directory_path,omitempty"`

	RequestBodyMemoryBufferSize *int64 `json:"request_body_memory_buffer_size,omitempty"`
	MaxRequestBodySize          *int64 `json:"max_request_body_size,omitempty"`

	ResponseBodyMemoryBufferSize *int64 `json:"response_body_memory_buffer_size,omitempty"`
	MaxResponseBodySize          *int64 `json:"max_response_body_size,omitempty"`

	RequestTimeout *float64 `json:"request_timeout,omitempty"` // seconds
}

func (cfg *FastCGIActionCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckNetworkAddress("address", cfg.Address)

	if cfg.RequestBodyMemoryBufferSize != nil {
		v.CheckInt64Min("request_body_memory_buffer_size",
			*cfg.RequestBodyMemoryBufferSize, 0)
	}

	if cfg.MaxRequestBodySize != nil {
		v.CheckInt64Min("max_request_body_size", *cfg.MaxRequestBodySize, 0)
	}

	if cfg.ResponseBodyMemoryBufferSize != nil {
		v.CheckInt64Min("response_body_memory_buffer_size",
			*cfg.ResponseBodyMemoryBufferSize, 0)
	}

	if cfg.MaxResponseBodySize != nil {
		v.CheckInt64Min("max_response_body_size", *cfg.MaxResponseBodySize, 0)
	}

	if timeout := cfg.RequestTimeout; timeout != nil {
		v.CheckFloatMin("request_timeout", *cfg.RequestTimeout, 0.0)
	}
}

type FastCGIAction struct {
	Handler *Handler
	Cfg     *FastCGIActionCfg
	Log     *log.Logger

	client *fastcgi.Client

	scriptRE *regexp.Regexp

	tmpDirPath string

	reqBodyMemBufSize int64
	maxReqBodySize    int64

	resBodyMemBufSize int64
	maxResBodySize    int64

	requestTimeout time.Duration
}

func NewFastCGIAction(h *Handler, cfg *FastCGIActionCfg) (*FastCGIAction, error) {
	a := FastCGIAction{
		Handler: h,
		Cfg:     cfg,
		Log:     h.Module.Log,
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

	a.tmpDirPath = cfg.TemporaryDirectoryPath
	if a.tmpDirPath == "" {
		dirPath, err := os.MkdirTemp("", "boulevard-fastcgi-*")
		if err != nil {
			return nil, fmt.Errorf("cannot create temporary directory: %w", err)
		}

		a.tmpDirPath = dirPath
	}

	a.reqBodyMemBufSize = DefaultRequestBodyMemoryBufferSize
	if size := cfg.RequestBodyMemoryBufferSize; size != nil {
		a.reqBodyMemBufSize = *size
	}

	a.maxReqBodySize = DefaultMaxRequestBodySize
	if size := cfg.MaxRequestBodySize; size != nil {
		a.maxReqBodySize = *size
	}

	a.resBodyMemBufSize = DefaultResponseBodyMemoryBufferSize
	if size := cfg.ResponseBodyMemoryBufferSize; size != nil {
		a.resBodyMemBufSize = *size
	}

	a.maxResBodySize = DefaultMaxResponseBodySize
	if size := cfg.MaxResponseBodySize; size != nil {
		a.maxResBodySize = *size
	}

	requestTimeout := DefaultRequestTimeout
	if timeout := cfg.RequestTimeout; timeout != nil {
		requestTimeout = *timeout
	}
	a.requestTimeout = time.Duration(requestTimeout * float64(time.Second))

	return &a, nil
}

func (a *FastCGIAction) Start() error {
	clientCfg := fastcgi.ClientCfg{
		Log:     a.Log,
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

	if err := os.RemoveAll(a.tmpDirPath); err != nil {
		a.Log.Error("cannot delete directory %q: %v", a.tmpDirPath, err)
	}
}

func (a *FastCGIAction) HandleRequest(ctx *RequestContext) {
	// In theory we should be able to stream the request body, which would be
	// nice if the client used chunked transfer encoding. But if we do not
	// buffer the request body, we have no way to compute its length, meaning we
	// cannot set the mandatory CONTENT_LENGTH FastCGI parameter.
	reqBodyBuf := a.newRequestBodySpillBuffer()
	defer func() {
		if err := reqBodyBuf.Close(); err != nil {
			ctx.Log.Error("cannot close spill buffer: %v", err)
		}
	}()

	reqBodySize, err := io.Copy(reqBodyBuf, ctx.Request.Body)
	if err != nil {
		ctx.Log.Error("cannot copy request body: %v", err)
		ctx.ReplyError(500)
		return
	}

	params := a.requestParameters(ctx, reqBodySize)
	stdin, err := reqBodyBuf.Reader()
	if err != nil {
		ctx.Log.Error("cannot read spill buffer: %v", err)
		ctx.ReplyError(500)
		return
	}
	defer stdin.Close()

	// We have to buffer the response body since we need to send a
	// Content-Length header field. We could use chunked transfer encoding if
	// the client supports HTTP 1.1 but that would force FastCGI applications to
	// send the content length as a response header field (something they do
	// not), and we would still have to implement buffering for HTTP 1.0
	// clients.
	resBodyBuf := a.newResponseBodySpillBuffer()
	defer func() {
		if err := resBodyBuf.Close(); err != nil {
			ctx.Log.Error("cannot close spill buffer: %v", err)
		}
	}()

	var stderr bytes.Buffer

	timeoutCtx, cancelTimeoutCtx := context.WithTimeout(ctx.Ctx,
		a.requestTimeout)
	defer cancelTimeoutCtx()

	resHeader, err := a.client.SendRequest(timeoutCtx, fastcgi.RoleResponder,
		params, stdin, nil, resBodyBuf, &stderr)
	if err != nil {
		if !netutils.IsConnectionClosedError(err) {
			ctx.Log.Error("cannot send FastCGI request: %v", err)
		}

		status := 500
		if errors.Is(err, fastcgi.ErrServerOverloaded) {
			status = 503
		} else if errors.Is(err, fastcgi.ErrRequestTimeout) {
			status = 504
		}

		ctx.ReplyError(status)
		return
	}

	if stderr.Len() > 0 {
		ctx.Log.Error("FastCGI error: %s", stderr.String())
	}

	resBodyReader, err := resBodyBuf.Reader()
	if err != nil {
		ctx.Log.Error("cannot read spill buffer: %v", err)
		ctx.ReplyError(500)
		return
	}
	defer resBodyReader.Close()

	header := ctx.ResponseWriter.Header()
	resHeader.CopyToHTTPHeader(header)

	header.Set("Content-Length", strconv.FormatInt(resBodyBuf.Size(), 10))

	// It would be nice to be able to use the reason string, but the
	// http.ResponseWriter interface does not support it.
	statusCode, _ := resHeader.Status()
	ctx.Reply(statusCode, resBodyReader)
}

func (a *FastCGIAction) newRequestBodySpillBuffer() *boulevard.SpillBuffer {
	fileName := hex.EncodeToString(boulevard.RandomBytes(16))
	filePath := path.Join(a.tmpDirPath, fileName)

	return boulevard.NewSpillBuffer(filePath, a.reqBodyMemBufSize,
		a.maxReqBodySize)
}

func (a *FastCGIAction) newResponseBodySpillBuffer() *boulevard.SpillBuffer {
	fileName := hex.EncodeToString(boulevard.RandomBytes(16))
	filePath := path.Join(a.tmpDirPath, fileName)

	return boulevard.NewSpillBuffer(filePath, a.resBodyMemBufSize,
		a.maxResBodySize)
}

func (a *FastCGIAction) requestParameters(ctx *RequestContext, reqBodySize int64) fastcgi.NameValuePairs {
	req := ctx.Request
	header := req.Header

	var pathInfo string
	var scriptName string // must not start with '/'

	subpath := ctx.Subpath // relative
	if subpath == "" {
		subpath = a.Cfg.DefaultScript
	}

	if a.scriptRE == nil {
		scriptName = subpath
		pathInfo = "/"
	} else {
		if match := a.scriptRE.FindString("/" + subpath); match == "" {
			scriptName = subpath
			pathInfo = "/"
		} else {
			scriptName = strings.TrimPrefix(match, "/")
			pathInfo = path.Join("/", strings.TrimPrefix(subpath, match))
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
		"CONTENT_LENGTH":    strconv.FormatInt(reqBodySize, 10),
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
