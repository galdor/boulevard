package main

import (
	"go.n16f.net/program"
)

func cmdValues(p *program.Program) {
	address := p.ArgumentValue("address")

	client := newClient(p, address)

	t := program.NewKeyValueTable()
	for _, pair := range client.Values() {
		t.AddRow(pair.Name, pair.Value)
	}

	t.Print()
}
