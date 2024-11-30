package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"go.n16f.net/boulevard/pkg/modules/httpserver"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ControlAPICfg struct {
	Address      string                      `json:"address"`
	TLS          *netutils.TLSCfg            `json:"tls,omitempty"`
	Auth         *httpserver.AuthCfg         `json:"authentication,omitempty"`
	AccessLogger *httpserver.AccessLoggerCfg `json:"access_logs,omitempty"`
}

func (cfg *ControlAPICfg) ValidateJSON(v *ejson.Validator) {
	v.CheckNetworkAddress("address", cfg.Address)
	v.CheckOptionalObject("tls", cfg.TLS)
	v.CheckOptionalObject("authentication", cfg.Auth)
	v.CheckOptionalObject("access_logs", cfg.AccessLogger)
}

type ControlAPI struct {
	Cfg     *ControlAPICfg
	Log     *log.Logger
	Service *Service

	tcpListener  *netutils.TCPListener
	httpServer   *http.Server
	accessLogger *httpserver.AccessLogger
	auth         httpserver.Auth

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (s *Service) initControlAPI() error {
	cfg := s.Cfg.ControlAPI
	if cfg == nil {
		return nil
	}

	s.Log.Debug(1, "starting control API")

	logger := s.Log.Child("control_api", nil)

	tcpListenerCfg := netutils.TCPListenerCfg{
		Address:    cfg.Address,
		TLS:        cfg.TLS,
		Log:        logger,
		ACMEClient: s.acmeClient,
	}

	tcpListener, err := netutils.NewTCPListener(tcpListenerCfg)
	if err != nil {
		return fmt.Errorf("cannot create TCP listener: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	api := ControlAPI{
		Cfg:     cfg,
		Log:     logger,
		Service: s,

		tcpListener: tcpListener,

		ctx:    ctx,
		cancel: cancel,
	}

	if logCfg := cfg.AccessLogger; logCfg != nil {
		log, err := httpserver.NewAccessLogger(logCfg, nil)
		if err != nil {
			return fmt.Errorf("cannot create access logger: %w", err)
		}

		api.accessLogger = log
	}

	if authCfg := cfg.Auth; authCfg != nil {
		auth, err := httpserver.NewAuth(authCfg)
		if err != nil {
			if api.accessLogger != nil {
				api.accessLogger.Close()
			}
			return fmt.Errorf("cannot initialize authentication: %w", err)
		}

		api.auth = auth
	}

	s.controlAPI = &api

	return nil
}

func (s *Service) startControlAPI() error {
	if s.controlAPI == nil {
		return nil
	}

	s.Log.Debug(1, "starting control API")

	if err := s.controlAPI.Start(); err != nil {
		return fmt.Errorf("cannot start control API: %w", err)
	}

	return nil
}

func (s *Service) stopControlAPI() {
	if s.controlAPI == nil {
		return
	}

	s.Log.Debug(1, "stopping control API")

	s.controlAPI.Stop()
}

func (api *ControlAPI) Start() error {
	if err := api.tcpListener.Start(); err != nil {
		return fmt.Errorf("cannot start TCP listener: %w", err)
	}

	api.httpServer = &http.Server{
		Addr:     api.Cfg.Address,
		Handler:  api,
		ErrorLog: api.Log.StdLogger(log.LevelError),
	}

	api.wg.Add(1)
	go api.serve()

	return nil
}

func (api *ControlAPI) Stop() {
	api.cancel()
	api.httpServer.Shutdown(api.ctx)
	api.wg.Wait()

	api.tcpListener.Stop()
}

func (api *ControlAPI) serve() {
	defer api.wg.Done()

	err := api.httpServer.Serve(api.tcpListener.Listener)
	if err != http.ErrServerClosed {
		api.Log.Error("cannot run HTTP server: %v", err)
		return
	}
}

func (api *ControlAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := httpserver.NewRequestContext(api.ctx, req, w)
	ctx.Log = api.Log.Child("", nil)
	ctx.AccessLogger = api.accessLogger
	ctx.Auth = api.auth

	defer ctx.Recover()
	defer ctx.OnRequestHandled()

	if err := ctx.IdentifyClient(); err != nil {
		ctx.Log.Error("cannot identify client: %v", err)
		ctx.ReplyError(500)
		return
	}

	if err := ctx.IdentifyRequestHost(); err != nil {
		ctx.Log.Error("cannot identify request host: %v", err)
		ctx.ReplyError(500)
		return
	}

	if ctx.Auth != nil {
		if err := ctx.Auth.AuthenticateRequest(ctx); err != nil {
			ctx.Log.Error("cannot authenticate request: %v", err)
			return
		}
	}

	switch ctx.Request.Method {
	case "GET":
	case "POST":
	default:
		header := ctx.ResponseWriter.Header()
		header.Set("Allow", "GET, POST")
		ctx.ReplyError(405)
		return
	}

	op := strings.Trim(ctx.Request.URL.Path, "/")

	switch op {
	default:
		ctx.ReplyError(404)
	}
}
