package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/modules/httpserver"
	"go.n16f.net/log"
)

type ControlAPICfg struct {
	Path         string
	AccessLogger *httpserver.AccessLoggerCfg
}

func (cfg *ControlAPICfg) Init(block *bcl.Element) {
	block.EntryValue("path", &cfg.Path)

	if block := block.MaybeBlock("access_logs"); block != nil {
		cfg.AccessLogger = new(httpserver.AccessLoggerCfg)
		cfg.AccessLogger.Init(block)
	}
}

type ControlAPI struct {
	Cfg     *ControlAPICfg
	Log     *log.Logger
	Service *Service

	httpServer   *http.Server
	accessLogger *httpserver.AccessLogger

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

	ctx, cancel := context.WithCancel(context.Background())

	api := ControlAPI{
		Cfg:     cfg,
		Log:     logger,
		Service: s,

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
	listener, err := net.Listen("unix", api.Cfg.Path)
	if err != nil {
		return fmt.Errorf("cannot listen on %q: %w", api.Cfg.Path, err)
	}

	api.Log.Info("listening on %q", api.Cfg.Path)

	api.httpServer = &http.Server{
		Handler:  api,
		ErrorLog: api.Log.StdLogger(log.LevelError),
	}

	api.wg.Add(1)
	go api.serve(listener)

	return nil
}

func (api *ControlAPI) Stop() {
	api.cancel()
	api.httpServer.Shutdown(api.ctx)
	api.wg.Wait()
}

func (api *ControlAPI) serve(listener net.Listener) {
	defer api.wg.Done()
	defer listener.Close()

	err := api.httpServer.Serve(listener)
	if err != http.ErrServerClosed {
		api.Log.Error("cannot run HTTP server: %v", err)
		return
	}
}

func (api *ControlAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := httpserver.NewRequestContext(api.ctx, req, w)
	ctx.Log = api.Log.Child("", nil)
	ctx.AccessLogger = api.accessLogger

	defer ctx.Recover()
	defer ctx.OnRequestHandled()

	switch ctx.Request.Method {
	case "GET":
	case "POST":
	default:
		header := ctx.ResponseWriter.Header()
		header.Set("Allow", "GET, POST")
		ctx.ReplyError(405)
		return
	}

	h := ControlAPIHandler{
		Ctx: ctx,
	}

	op := strings.Trim(ctx.Request.URL.Path, "/")

	switch op {
	case "status":
		api.hStatus(&h)

	default:
		h.ReplyError(404, "unknown_operation", "unknown operation")
	}
}
