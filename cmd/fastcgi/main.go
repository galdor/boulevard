package main

import (
	"go.n16f.net/boulevard/pkg/fastcgi"
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

	c = p.AddCommand("send-request", "send a request and print the response",
		cmdSendRequest)
	c.AddArgument("address", "the address of the server")
	c.AddTrailingArgument("parameter",
		"a request parameter formatted as \"<name>=<value>\"")
	c.AddFlag("", "header", "print the response header")
	c.AddOption("r", "role", "role", string(fastcgi.RoleResponder),
		"the FastCGI role of the server")
	c.AddFlag("", "stdin", "send the standard input as request input")

	p.ParseCommandLine()
	p.Run()
}
