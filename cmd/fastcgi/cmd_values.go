package main

import (
	"go.n16f.net/boulevard/pkg/fastcgi"
	"go.n16f.net/log"
	"go.n16f.net/program"
)

func cmdValues(p *program.Program) {
	address := p.ArgumentValue("address")

	logger := log.DefaultLogger("fastcgi")
	logger.DebugLevel = p.DebugLevel

	clientCfg := fastcgi.ClientCfg{
		Log:     logger,
		Address: address,
	}

	client, err := fastcgi.NewClient(&clientCfg)
	if err != nil {
		p.Fatal("cannot create client: %v", err)
	}

	t := program.NewKeyValueTable()
	for _, pair := range client.Values() {
		t.AddRow(pair.Name, pair.Value)
	}

	t.Print()
}
