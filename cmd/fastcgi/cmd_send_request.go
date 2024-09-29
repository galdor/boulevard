package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

	res, err := client.SendRequest(role, params, stdin, nil)
	if err != nil {
		p.Fatal("cannot send request: %v", err)
	}

	if p.IsOptionSet("header") {
		for _, field := range res.Header.Fields {
			fmt.Printf("%s: %s\n", field.Name, field.Value)
		}

		fmt.Println()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case event := <-res.Events:
			if event == nil {
				return
			}

			if event.Error != nil {
				p.Fatal("cannot read response: %v", event.Error)
			}

			os.Stdout.Write(event.Data)

		case signo := <-sigChan:
			fmt.Fprintln(os.Stderr)
			p.Info("received signal %d (%v)", signo, signo)
		}
	}
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
