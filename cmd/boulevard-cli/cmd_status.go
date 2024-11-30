package main

import (
	"fmt"

	"go.n16f.net/boulevard/pkg/service"
	"go.n16f.net/program"
)

func cmdStatus(p *program.Program) {
	var status service.ServiceStatus
	if _, err := client.Call("status", nil, &status); err != nil {
		p.Fatal("cannot fetch server status: %v", err)
	}

	fmt.Println(status.BuildId)
}
