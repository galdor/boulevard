package http

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"go.n16f.net/bcl"
)

type BasicAuthCfg struct {
	Users        []string
	UserFilePath string
}

func (cfg *BasicAuthCfg) ReadBCLElement(block *bcl.Element) error {
	block.CheckEntriesOneOf("user", "user_file_path")

	for _, entry := range block.FindEntries("user") {
		var username, password string
		entry.Values(&username, &password)
		cfg.Users = append(cfg.Users, username+":"+password)
	}

	block.MaybeEntryValues("user_file_path", &cfg.UserFilePath)

	return nil
}

type BasicAuth struct {
	Cfg   *AuthCfg
	Users map[string]struct{}
}

func (a *BasicAuth) Init(cfg *AuthCfg) error {
	a.Cfg = cfg

	basicCfg := a.Cfg.Basic

	if filePath := basicCfg.UserFilePath; filePath == "" {
		a.Users = make(map[string]struct{})
		for _, u := range basicCfg.Users {
			a.Users[u] = struct{}{}
		}
	} else {
		if err := a.loadUsers(filePath); err != nil {
			return fmt.Errorf("cannot load user credentials: %w", err)
		}
	}

	return nil
}

func (a *BasicAuth) AuthenticateRequest(ctx *RequestContext) error {
	authorization := ctx.Request.Header.Get("Authorization")
	if authorization == "" {
		a.setWWWAuthenticate(ctx)
		err := errors.New("missing or empty Authorization header field")
		ctx.ReplyError2(401, "%v", err)
		return err
	}

	space := strings.IndexByte(authorization, ' ')
	if space == -1 {
		a.setWWWAuthenticate(ctx)
		err := errors.New("invalid authorization format")
		ctx.ReplyError2(401, "%v", err)
		return err
	}

	scheme := authorization[:space]

	if strings.ToLower(scheme) != "basic" {
		a.setWWWAuthenticate(ctx)
		err := fmt.Errorf("invalid authorization scheme %q", scheme)
		ctx.ReplyError2(401, "%v", err)
		return err
	}

	credentialsData, err := base64.StdEncoding.DecodeString(
		authorization[space+1:])
	if err != nil {
		err := fmt.Errorf("cannot decode base64-encoded credentials")
		ctx.ReplyError2(403, "%v", err)
		return err
	}

	username, password, found := strings.Cut(string(credentialsData), ":")
	if !found {
		err := fmt.Errorf("invalid authorization: missing ':' separator")
		ctx.ReplyError2(403, "%v", err)
		return err
	}

	ctx.Username = username
	ctx.Vars["http.request.username"] = username

	credentials := username + ":" + transformAuthSecret(password, a.Cfg)

	if _, found := a.Users[credentials]; !found {
		err := fmt.Errorf("invalid credentials")
		ctx.ReplyError2(403, "%v", err)
		return err
	}

	return nil
}

func (a *BasicAuth) loadUsers(filePath string) error {
	credentials, err := loadAuthSecretFile(filePath)
	if err != nil {
		return err
	}

	a.Users = credentials
	return nil
}

func (a *BasicAuth) setWWWAuthenticate(ctx *RequestContext) {
	value := "Basic"

	if realm := a.Cfg.Realm; realm != "" {
		value += fmt.Sprintf(" realm=%q", realm)
	}

	header := ctx.ResponseWriter.Header()
	header.Set("WWW-Authenticate", value)
}
