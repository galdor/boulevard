package main

import (
	"fmt"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/program"
)

func cmdVersion(p *program.Program) {
	version, err := boulevard.Version(buildId)
	if err != nil {
		p.Fatal("%v", err)
	}

	fmt.Println(version)
}
