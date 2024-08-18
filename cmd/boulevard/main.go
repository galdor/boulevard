package main

import (
	"go.n16f.net/program"
)

var buildId string

func main() {
	var c *program.Command

	program := program.NewProgram("boulevard", "a polyvalent reverse proxy")

	c = program.AddCommand("run", "run the service", cmdRun)
	c.AddOption("c", "cfg", "path", "", "the path of the configuration file")

	program.AddCommand("version", "print the version of the service and exit",
		cmdVersion)

	program.ParseCommandLine()
	program.Run()
}
