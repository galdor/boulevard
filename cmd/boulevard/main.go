package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/service"
	"go.n16f.net/program"
)

var buildId string

func main() {

	p := program.NewProgram("boulevard", "a polyvalent reverse proxy")

	p.AddOption("c", "cfg-file", "path", "",
		"the path of the configuration file")
	p.AddFlag("", "validate-cfg",
		"validate the configuration and exit")
	p.AddFlag("v", "version",
		"print the build identifier and exit")

	p.SetMain(run)

	p.ParseCommandLine()
	p.Run()
}

func run(p *program.Program) {
	// Command line
	if p.IsOptionSet("version") {
		version, err := boulevard.Version(buildId)
		if err != nil {
			p.Fatal("%v", err)
		}

		fmt.Println(version)
		return
	}

	cfgPath := p.OptionValue("cfg-file")

	// Configuration
	var cfg service.ServiceCfg

	cfg.BuildId = buildId
	cfg.ProtocolInfo = service.DefaultProtocols

	if cfgPath != "" {
		p.Info("loading configuration file %q", cfgPath)

		if err := cfg.Load(cfgPath); err != nil {
			p.Fatal("cannot load configuration from %q: %v", cfgPath, err)
		}
	}

	if p.IsOptionSet("validate-cfg") {
		p.Info("configuration validated successfully")
		return
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
	signal.Ignore(syscall.SIGPIPE)

	select {
	case signo := <-sigChan:
		fmt.Fprintln(os.Stderr)
		p.Info("received signal %d (%v)", signo, signo)
	}

	service.Stop()
}
