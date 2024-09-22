package main

import (
	"go.n16f.net/boulevard/pkg/fastcgi"
	"go.n16f.net/log"
	"go.n16f.net/program"
)

func cmdValues(p *program.Program) {
	address := p.ArgumentValue("address")

	clientCfg := fastcgi.ClientCfg{
		Log:     log.DefaultLogger("fastcgi"),
		Address: address,
	}

	client, err := fastcgi.NewClient(&clientCfg)
	if err != nil {
		p.Fatal("cannot create client: %v", err)
	}

	names := []string{
		"FCGI_MAX_CONNS",
		"FCGI_MAX_REQS",
		"FCGI_MPXS_CONNS",
	}

	pairs, err := client.FetchValues(names)
	if err != nil {
		p.Fatal("cannot fetch values: %v", err)
	}

	t := program.NewKeyValueTable()
	for _, pair := range pairs {
		t.AddRow(pair.Name, pair.Value)
	}

	t.Print()
}
