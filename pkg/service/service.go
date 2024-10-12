package service

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"sync"

	"go.n16f.net/acme"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/log"
)

type Service struct {
	Cfg ServiceCfg
	Log *log.Logger

	moduleInfo   map[string]*boulevard.ModuleInfo
	modules      map[string]*Module
	modulesMutex sync.Mutex

	acmeClient *acme.Client

	httpUserAgent string

	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewService(cfg ServiceCfg) (*Service, error) {
	var logger *log.Logger
	if cfg.Logger == nil {
		logger = log.DefaultLogger("boulevard")
	} else {
		var err error
		logger, err = log.NewLogger("boulevard", *cfg.Logger)
		if err != nil {
			return nil, fmt.Errorf("cannot create logger: %w", err)
		}
	}

	s := Service{
		Cfg: cfg,
		Log: logger,

		moduleInfo: make(map[string]*boulevard.ModuleInfo),
		modules:    make(map[string]*Module),

		stopChan: make(chan struct{}),
	}

	s.httpUserAgent = "boulevard/" + cfg.BuildId

	if err := s.initACMEClient(); err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *Service) Start() error {
	s.Log.Debug(1, "starting")

	if err := s.startACMEClient(); err != nil {
		return err
	}

	s.startPProf()

	if err := s.startModules(); err != nil {
		return err
	}

	s.Log.Debug(1, "running")
	return nil
}

func (s *Service) Stop() {
	s.Log.Debug(1, "stopping")

	close(s.stopChan)
	s.wg.Wait()

	s.stopModules()
	s.stopACMEClient()
}

func (s *Service) startPProf() {
	address := s.Cfg.PProfAddress
	if address == "" {
		return
	}

	go func() {
		if err := http.ListenAndServe(address, nil); err != nil {
			s.Log.Error("cannot start pprof: %v", err)
		}
	}()
}
