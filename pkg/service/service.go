package service

import (
	"fmt"
	"sync"

	"go.n16f.net/log"
)

type Service struct {
	Cfg ServiceCfg
	Log *log.Logger

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

		stopChan: make(chan struct{}),
	}

	return &s, nil
}

func (s *Service) Start() error {
	s.Log.Debug(1, "starting")

	s.Log.Debug(1, "running")
	return nil
}

func (s *Service) Stop() {
	s.Log.Debug(1, "stopping")

	close(s.stopChan)
	s.wg.Wait()
}
