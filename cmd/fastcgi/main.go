package main

import (
	"go.n16f.net/program"
)

var buildId string

func main() {
	var c *program.Command

	p := program.NewProgram("fastcgi", "FastCGI utilities")

	c = p.AddCommand("values",
		"print standard FastCGI values returned by a server",
		cmdValues)
	c.AddArgument("address", "the address of the server")

	p.ParseCommandLine()
	p.Run()
}
