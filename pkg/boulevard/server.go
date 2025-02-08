package boulevard

import (
	"fmt"

	"go.n16f.net/acme"
	"go.n16f.net/bcl"
	"go.n16f.net/log"
)

type ServerCfg struct {
	Listeners []*ListenerCfg

	Name             string
	Protocol         Protocol
	ProtocolInfo     *ProtocolInfo
	ProtocolCfg      ProtocolCfg
	ErrChan          chan<- error
	Log              *log.Logger
	BoulevardBuildId string
	ACMEClient       *acme.Client
	ServerStatuses   func() map[string]*ServerStatus
}

func (cfg *ServerCfg) ReadBCLElement(block *bcl.Element) error {
	block.Blocks("listener", &cfg.Listeners)
	return nil
}

type ServerStatus struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Data     any    `json:"data"`
}

type Server struct {
	Cfg       *ServerCfg
	Log       *log.Logger
	Listeners []*Listener

	stopChan chan struct{}
}

func StartServer(cfg *ServerCfg) (*Server, error) {
	s := Server{
		Cfg: cfg,
		Log: cfg.Log,

		stopChan: make(chan struct{}),
	}

	s.Listeners = make([]*Listener, len(cfg.Listeners))
	for i, listenerCfg := range cfg.Listeners {
		listenerCfg.Log = cfg.Log.Child("listener", nil)
		listenerCfg.ACMEClient = cfg.ACMEClient

		listener, err := StartListener(&s, listenerCfg)
		if err != nil {
			for j := range i {
				s.Listeners[j].Stop()
			}

			return nil, fmt.Errorf("cannot start listener: %w", err)
		}

		s.Listeners[i] = listener
	}

	if err := s.Cfg.Protocol.Start(&s); err != nil {
		for _, l := range s.Listeners {
			l.Stop()
		}
	}

	return &s, nil
}

func (s *Server) Stop() {
	close(s.stopChan)

	s.Cfg.Protocol.Stop()

	for _, l := range s.Listeners {
		l.Stop()
	}
}

func (s *Server) Fatal(format string, args ...any) {
	err := fmt.Errorf(format, args...)

	select {
	case s.Cfg.ErrChan <- err:
	case <-s.stopChan:
	}
}
