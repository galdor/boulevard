package boulevard

import (
	"fmt"

	"go.n16f.net/acme/pkg/acme"
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
	LoadBalancers    map[string]*LoadBalancer
}

func (cfg *ServerCfg) ReadBCLElement(block *bcl.Element) error {
	block.Blocks("listener", &cfg.Listeners)
	return nil
}

type ServerStatus struct {
	Name         string            `json:"name"`
	Listeners    []*ListenerStatus `json:"listeners"`
	Protocol     string            `json:"protocol"`
	ProtocolData any               `json:"protocol_data"`
}

type ListenerStatus struct {
	Address     string   `json:"address"`
	TLS         bool     `json:"tls"`
	ACME        bool     `json:"acme"`
	ACMEDomains []string `json:"acme_domains,omitempty"`
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

		return nil, err
	}

	return &s, nil
}

func (s *Server) Stop() {
	close(s.stopChan)

	// Stop listening first, then let protocol implementations close any
	// connection: if a connection is accepted after closing existing ones but
	// before stopping listeners, we will leak its file descriptor.
	//
	// This also ensures that goroutines running Accept on these listeners (see
	// for example the protocols/tcp package) can be properly interrupted before
	// the Stop method of the protocol returns, avoiding interesting race
	// conditions.

	for _, l := range s.Listeners {
		l.Stop()
	}

	s.Cfg.Protocol.Stop()
}

func (s *Server) Fatal(format string, args ...any) {
	err := fmt.Errorf(format, args...)

	select {
	case s.Cfg.ErrChan <- err:
	case <-s.stopChan:
	}
}
