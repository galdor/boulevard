package service

import (
	"fmt"
	"time"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/log"
)

func (s *Service) startServers() error {
	var startedServers []string

	s.servers = make(map[string]*boulevard.Server)

	for _, serverCfg := range s.Cfg.Servers {
		if err := s.startServer(serverCfg); err != nil {
			for _, name := range startedServers {
				s.stopServer(name)
			}

			return err
		}

		startedServers = append(startedServers, serverCfg.Name)
	}

	return nil
}

func (s *Service) startServer(cfg *boulevard.ServerCfg) error {
	if _, found := s.servers[cfg.Name]; found {
		return fmt.Errorf("duplicate server %q", cfg.Name)
	}

	s.Log.Debug(1, "starting server %q", cfg.Name)

	errChan := make(chan error)

	cfg.ErrChan = errChan
	cfg.Log = s.Log.Child("server", log.Data{"server": cfg.Name})
	cfg.ACMEClient = s.acmeClient
	cfg.ServerStatuses = s.serverStatuses

	s.Log.Debug(1, "starting server %q", cfg.Name)

	server, err := boulevard.StartServer(cfg)
	if err != nil {
		close(errChan)
		return fmt.Errorf("cannot start server: %w", err)
	}

	go func() {
		if err := <-errChan; err != nil {
			s.handleServerError(cfg.Name, err)
		}
	}()

	s.servers[cfg.Name] = server

	return nil
}

func (s *Service) stopServers() {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	for name := range s.servers {
		s.stopServer(name)
	}
}

func (s *Service) stopServer(name string) {
	server, found := s.servers[name]
	if !found {
		return
	}

	s.Log.Debug(1, "stopping server %q", name)

	server.Stop()
	delete(s.servers, name)

	close(server.Cfg.ErrChan)
}

func (s *Service) handleServerError(name string, err error) {
	select {
	case <-s.stopChan:
		return
	default:
	}

	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	server := s.servers[name]

	server.Log.Error("fatal: %v", err)
	s.stopServer(name)

	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
			case <-s.stopChan:
				return
			}

			func() {
				s.serverMutex.Lock()
				defer s.serverMutex.Unlock()

				if err := s.startServer(server.Cfg); err != nil {
					server.Log.Error("cannot restart server %s: %v", name, err)
				}
			}()

			return
		}
	}()
}

func (s *Service) serverStatuses() map[string]*boulevard.ServerStatus {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	statuses := make(map[string]*boulevard.ServerStatus)
	for name, server := range s.servers {
		statuses[name] = &boulevard.ServerStatus{
			Name:     server.Cfg.Name,
			Protocol: server.Cfg.ProtocolInfo.Name,
			Data:     server.Cfg.Protocol.StatusData(),
		}
	}

	return statuses
}
