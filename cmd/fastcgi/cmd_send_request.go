package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.n16f.net/boulevard/pkg/fastcgi"
	"go.n16f.net/program"
)

func cmdSendRequest(p *program.Program) {
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

	client := newClient(p, address)

	status, err := client.SendRequest(role, params, stdin, nil, os.Stdout,
		os.Stderr)
	if err != nil {
		p.Fatal("cannot send request: %v", err)
	}

	os.Exit(int(status))
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
