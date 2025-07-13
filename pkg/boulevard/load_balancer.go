package boulevard

import (
	"fmt"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/netutils"
)

type LoadBalancerCfg struct {
	Name    string
	Servers []netutils.HostAddress
}

func (cfg *LoadBalancerCfg) ReadBCLElement(block *bcl.Element) error {
	for _, elt := range block.FindEntries("server") {
		var host netutils.HostAddress
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
	Address netutils.HostAddress
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
	for i, address := range cfg.Servers {
		s := LoadBalancerServer{
			Address: address,
		}

		lb.Servers[i] = &s
	}

	return &lb, nil
}

func (lb *LoadBalancer) Stop() {
}

func (lb *LoadBalancer) Address() string {
	server := lb.Servers[lb.NextServerIndex]
	lb.NextServerIndex = (lb.NextServerIndex + 1) % len(lb.Servers)

	return server.Address.String()
}
