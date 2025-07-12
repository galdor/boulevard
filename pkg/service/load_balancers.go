package service

import (
	"fmt"

	"go.n16f.net/boulevard/pkg/boulevard"
)

func (s *Service) startLoadBalancers() error {
	var startedLoadBalancers []string

	s.loadBalancers = make(map[string]*boulevard.LoadBalancer)

	for _, loadBalancerCfg := range s.Cfg.LoadBalancers {
		if err := s.startLoadBalancer(loadBalancerCfg); err != nil {
			for _, name := range startedLoadBalancers {
				s.stopLoadBalancer(name)
			}

			return err
		}

		startedLoadBalancers = append(startedLoadBalancers,
			loadBalancerCfg.Name)
	}

	return nil
}

func (s *Service) startLoadBalancer(cfg *boulevard.LoadBalancerCfg) error {
	if _, found := s.loadBalancers[cfg.Name]; found {
		return fmt.Errorf("duplicate load balancer %q", cfg.Name)
	}

	s.Log.Debug(1, "starting load balancer %q", cfg.Name)

	loadBalancer, err := boulevard.StartLoadBalancer(cfg)
	if err != nil {
		return fmt.Errorf("cannot start load balancer %q: %w", cfg.Name, err)
	}

	s.loadBalancers[cfg.Name] = loadBalancer

	return nil
}

func (s *Service) stopLoadBalancers() {
	s.loadBalancerMutex.Lock()
	defer s.loadBalancerMutex.Unlock()

	for name := range s.loadBalancers {
		s.stopLoadBalancer(name)
	}
}

func (s *Service) stopLoadBalancer(name string) {
	loadBalancer, found := s.loadBalancers[name]
	if !found {
		return
	}

	s.Log.Debug(1, "stopping load balancer %q", name)

	loadBalancer.Stop()
	delete(s.loadBalancers, name)
}
