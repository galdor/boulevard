package boulevard

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
)

type LoadBalancerCfg struct {
	Name        string
	Servers     []netutils.HostAddress
	HealthProbe *HealthProbeCfg

	Log *log.Logger
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

	block.MaybeBlock("health_probe", &cfg.HealthProbe)

	return nil
}

type LoadBalancerServer struct {
	Address netutils.HostAddress

	healthy     atomic.Bool
	healthProbe *HealthProbe
}

type LoadBalancer struct {
	Cfg *LoadBalancerCfg
	Log *log.Logger

	Servers         []*LoadBalancerServer
	nextServerIndex int

	stopChan chan struct{}
	wg       sync.WaitGroup
}

func StartLoadBalancer(cfg *LoadBalancerCfg) (*LoadBalancer, error) {
	lb := LoadBalancer{
		Cfg: cfg,
		Log: cfg.Log,

		stopChan: make(chan struct{}),
	}

	lb.Servers = make([]*LoadBalancerServer, len(cfg.Servers))
	for i, address := range cfg.Servers {
		s := LoadBalancerServer{
			Address: address,
		}

		s.healthy.Store(true)
		if cfg.HealthProbe != nil {
			s.healthProbe = NewHealthProbe(s.Address.String(), cfg.HealthProbe)
		}

		lb.Servers[i] = &s
	}

	if cfg.HealthProbe != nil {
		for _, server := range lb.Servers {
			lb.wg.Add(1)
			go lb.watchServerHealth(server)
		}
	}

	return &lb, nil
}

func (lb *LoadBalancer) Stop() {
	close(lb.stopChan)
	lb.wg.Wait()
}

func (lb *LoadBalancer) watchServerHealth(server *LoadBalancerServer) {
	defer lb.wg.Done()

	probe := server.healthProbe

	logData := log.Data{
		"server": server.Address.String(),
	}

	period := time.Duration(lb.Cfg.HealthProbe.Period) * time.Second
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-lb.stopChan:
			return

		case <-ticker.C:
			wasHealthy := server.healthy.Load()
			healthy, err := probe.Execute()

			if wasHealthy && err != nil {
				lb.Log.InfoData(logData, "health test failure: %v", err)
			}

			switch {
			case wasHealthy && !healthy:
				lb.Log.InfoData(logData, "disabling unhealthy server")
				server.healthy.Store(false)

			case !wasHealthy && healthy:
				lb.Log.InfoData(logData, "re-enabling healthy server")
				server.healthy.Store(true)
			}
		}
	}
}

func (lb *LoadBalancer) Address() string {
	var server *LoadBalancerServer

	firstIndex := lb.nextServerIndex
	for {
		server = lb.Servers[lb.nextServerIndex]
		lb.nextServerIndex = (lb.nextServerIndex + 1) % len(lb.Servers)

		if server.healthy.Load() == true {
			return server.Address.String()
		}

		if lb.nextServerIndex == firstIndex {
			break
		}
	}

	// No healthy server available
	return ""
}
