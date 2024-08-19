package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.n16f.net/boulevard/pkg/boulevard"
	modhttpserver "go.n16f.net/boulevard/pkg/modules/httpserver"
	modtcpserver "go.n16f.net/boulevard/pkg/modules/tcpserver"
	"go.n16f.net/boulevard/pkg/service"
	"go.n16f.net/program"
)

func cmdRun(p *program.Program) {
	// Command line
	cfgPath := p.OptionValue("cfg")

	// Configuration
	var cfg service.ServiceCfg

	if cfgPath != "" {
		p.Info("loading configuration file %q", cfgPath)

		if err := cfg.Load(cfgPath); err != nil {
			p.Fatal("cannot load configuration from %q: %v", cfgPath, err)
		}
	}

	cfg.BuildId = buildId

	cfg.ModuleInfo = []*boulevard.ModuleInfo{
		modhttpserver.ModuleInfo(),
		modtcpserver.ModuleInfo(),
	}

	// Service
	service, err := service.NewService(cfg)
	if err != nil {
		p.Fatal("cannot create service: %v", err)
	}

	if err := service.Start(); err != nil {
		p.Fatal("cannot start service: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case signo := <-sigChan:
		fmt.Fprintln(os.Stderr)
		p.Info("received signal %d (%v)", signo, signo)
	}

	service.Stop()
}
