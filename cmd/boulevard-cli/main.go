package main

import (
	"go.n16f.net/boulevard/pkg/service"
	"go.n16f.net/program"
)

var (
	buildId string
	version string

	client *service.Client
)

func main() {
	version = buildId
	if version[0] == 'v' {
		version = version[1:]
	}

	p := program.NewProgram("boulevard-cli",
		"command line tool for the Boulevard reverse proxy")

	p.AddOption("p", "path", "path", "/var/run/boulevard.sock",
		"the UNIX socket to connect to")

	p.AddCommand("status", "print the status of the server", cmdStatus)
	p.AddCommand("version", "print the version of the client", cmdVersion)

	p.ParseCommandLine()

	if p.CommandName() != "version" {
		path := p.OptionValue("path")
		client = service.NewClient(path)
	}

	p.Run()
}
