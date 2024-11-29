package main

import (
	"go.n16f.net/program"
)

var buildId string

func main() {
	p := program.NewProgram("boulevard-cli",
		"command line tool for the Boulevard reverse proxy")

	p.AddOption("s", "server", "address", "localhost:4960",
		"the address of the server")

	p.AddCommand("version", "print the version of the client", cmdVersion)

	p.ParseCommandLine()
	p.Run()
}
