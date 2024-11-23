package httpserver

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

type BasicAuth struct {
	Cfg         *AuthCfg
	Credentials map[string]struct{}
}

func (a *BasicAuth) Init(cfg *AuthCfg) error {
	a.Cfg = cfg

	basicCfg := a.Cfg.Basic

	if filePath := basicCfg.CredentialFilePath; filePath == "" {
		a.Credentials = make(map[string]struct{})
		for _, c := range basicCfg.Credentials {
			a.Credentials[c] = struct{}{}
		}
	} else {
		if err := a.loadCredentials(filePath); err != nil {
			return fmt.Errorf("cannot load credentials: %w", err)
		}
	}

	return nil
}

func (a *BasicAuth) AuthenticateRequest(ctx *RequestContext) error {
	authorization := ctx.Request.Header.Get("Authorization")
	if authorization == "" {
		a.setWWWAuthenticate(ctx)
		ctx.ReplyError(401)
		return errors.New("missing or empty Authorization header field")
	}

	space := strings.IndexByte(authorization, ' ')
	if space == -1 {
		a.setWWWAuthenticate(ctx)
		ctx.ReplyError(401)
		return errors.New("invalid authorization format")
	}

	scheme := authorization[:space]

	if strings.ToLower(scheme) != "basic" {
		a.setWWWAuthenticate(ctx)
		ctx.ReplyError(401)
		return fmt.Errorf("invalid authorization scheme %q", scheme)
	}

	credentialsData, err := base64.StdEncoding.DecodeString(
		authorization[space+1:])
	if err != nil {
		ctx.ReplyError(403)
		return fmt.Errorf("cannot decode base64-encoded credentials")
	}

	username, password, found := strings.Cut(string(credentialsData), ":")
	if !found {
		ctx.ReplyError(403)
		return fmt.Errorf("invalid authorization: missing ':' separator")
	}

	ctx.Username = username
	ctx.Vars["http.request.username"] = username

	credentials := username + ":" + transformAuthSecret(password, a.Cfg)

	if _, found := a.Credentials[credentials]; !found {
		ctx.ReplyError(403)
		return fmt.Errorf("invalid credentials")
	}

	return nil
}

func (a *BasicAuth) loadCredentials(filePath string) error {
	credentials, err := loadAuthSecretFile(filePath)
	if err != nil {
		return err
	}

	a.Credentials = credentials
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
