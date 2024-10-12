package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"go.n16f.net/boulevard/pkg/fastcgi"
	"go.n16f.net/program"
)

func cmdSendRequest(p *program.Program) {
	ctx := context.Background()

	address := p.ArgumentValue("address")

	paramStrings := p.TrailingArgumentValues("parameter")
	params, err := parseNameValuePairs(paramStrings)
	if err != nil {
		p.Fatal("cannot parse parameters: %v", err)
	}

	roleString := p.OptionValue("role")
	role, err := parseRole(roleString)
	if err != nil {
		p.Fatal("cannot parse role: %v", err)
	}

	var stdin io.Reader
	if p.IsOptionSet("stdin") {
		stdin = os.Stdin
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	client := newClient(p, address)

	header, err := client.SendRequest(ctx, role, params, stdin, nil, &stdout,
		&stderr)
	if err != nil {
		p.Fatal("cannot send request: %v", err)
	}

	if stderr.Len() > 0 {
		p.Error("FastCGI error: %s", stderr.String())
	}

	if p.IsOptionSet("header") {
		for _, field := range header.Fields {
			fmt.Printf("%s: %s\n", field.Name, field.Value)
		}

		fmt.Println()
	}

	io.Copy(os.Stdout, &stdout)
}

func parseRole(s string) (fastcgi.Role, error) {
	var role fastcgi.Role

	switch s {
	case string(fastcgi.RoleAuthorizer):
		role = fastcgi.RoleAuthorizer
	case string(fastcgi.RoleFilter):
		role = fastcgi.RoleFilter
	case string(fastcgi.RoleResponder):
		role = fastcgi.RoleResponder
	default:
		return "", fmt.Errorf("unknown role %q", s)
	}

	return role, nil
}

func parseNameValuePairs(ss []string) (fastcgi.NameValuePairs, error) {
	var params fastcgi.NameValuePairs

	for _, s := range ss {
		var param fastcgi.NameValuePair
		param.Name, param.Value, _ = strings.Cut(s, "=")

		params = append(params, param)
	}

	return params, nil
}
