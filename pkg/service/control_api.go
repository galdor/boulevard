package service

import (
	"context"
	"fmt"
	"io"
	stdlog "log"
	"net"
	nethttp "net/http"
	"os"
	"strings"
	"sync"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/protocols/http"
	"go.n16f.net/log"
)

type ControlAPICfg struct {
	Path              string
	AccessLogger      *http.AccessLoggerCfg
	LogGoServerErrors bool
}

func (cfg *ControlAPICfg) ReadBCLElement(block *bcl.Element) error {
	block.EntryValues("path", &cfg.Path)
	block.MaybeBlock("access_logs", &cfg.AccessLogger)
	block.MaybeEntryValues("log_go_server_errors", &cfg.LogGoServerErrors)
	return nil
}

type ControlAPI struct {
	Cfg     *ControlAPICfg
	Log     *log.Logger
	Service *Service

	httpServer   *nethttp.Server
	accessLogger *http.AccessLogger

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
		log, err := http.NewAccessLogger(logCfg, nil)
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
	// UNIX socket remain after program termination, and no amount of shutdown
	// code will change the fact that the program (or the underlying machine)
	// crashing will lead to a stray socket file preventing Boulevard from
	// restarting.
	//
	// The only fix is to always delete existing socket files on startup. The
	// only downside is that you could accidentally mess with another running
	// Boulevard instance. Not really a problem for a daemon supposed to be
	// managed by the init system of the host.
	if err := os.Remove(api.Cfg.Path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot delete %q: %w", api.Cfg.Path, err)
		}
	}

	listener, err := net.Listen("unix", api.Cfg.Path)
	if err != nil {
		return fmt.Errorf("cannot listen on %q: %w", api.Cfg.Path, err)
	}

	api.Log.Info("listening on %q", api.Cfg.Path)

	api.httpServer = &nethttp.Server{
		Handler:  api,
		ErrorLog: stdlog.New(io.Discard, "", 0),
	}

	if api.Cfg.LogGoServerErrors {
		api.httpServer.ErrorLog = api.Log.StdLogger(log.LevelError)
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
	if err != nethttp.ErrServerClosed {
		api.Log.Error("cannot run HTTP server: %v", err)
		return
	}
}

func (api *ControlAPI) ServeHTTP(w nethttp.ResponseWriter, req *nethttp.Request) {
	ctx := http.NewRequestContext(api.ctx, req, w)
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
		ctx.ReplyError2(405, "HTTP method not supported")
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
