package boulevard

import (
	"fmt"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/netutils"
)

type LoadBalancerCfg struct {
	Name    string
	Servers []netutils.Host
}

func (cfg *LoadBalancerCfg) ReadBCLElement(block *bcl.Element) error {
	for _, elt := range block.FindEntries("server") {
		var host netutils.Host
		elt.Values(&host)
		cfg.Servers = append(cfg.Servers, host)
	}

	if len(cfg.Servers) == 0 {
		return fmt.Errorf("load balancer configuration does no contain " +
			"any server")
	}

	return nil
}

type LoadBalancerServer struct {
	Host netutils.Host
}

type LoadBalancer struct {
	Cfg *LoadBalancerCfg

	Servers         []*LoadBalancerServer
	NextServerIndex int
}

func StartLoadBalancer(cfg *LoadBalancerCfg) (*LoadBalancer, error) {
	lb := LoadBalancer{
		Cfg: cfg,
	}

	lb.Servers = make([]*LoadBalancerServer, len(cfg.Servers))
	for i, host := range cfg.Servers {
		s := LoadBalancerServer{
			Host: host,
		}

		lb.Servers[i] = &s
	}

	return &lb, nil
}

func (lb *LoadBalancer) Stop() {
}

func (lb *LoadBalancer) Address() (string, error) {
	server := lb.Servers[lb.NextServerIndex]
	lb.NextServerIndex = (lb.NextServerIndex + 1) % len(lb.Servers)

	if addr := server.Host.Address; addr != nil {
		return addr.String(), nil
	}

	return server.Host.Hostname, nil
}
