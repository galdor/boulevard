package main

import (
	"context"

	"go.n16f.net/program"
)

func cmdValues(p *program.Program) {
	ctx := context.Background()

	address := p.ArgumentValue("address")

	client := newClient(p, address)

	values, err := client.FetchValues(ctx)
	if err != nil {
		p.Fatal("cannot fetch values: %v", err)
	}

	t := program.NewKeyValueTable()
	for _, pair := range values {
		t.AddRow(pair.Name, pair.Value)
	}

	t.Print()
}
