package http

import (
	"errors"
	"fmt"
	"strings"

	"go.n16f.net/bcl"
)

type BearerAuthCfg struct {
	Tokens    []string
	TokenFile string
}

func (cfg *BearerAuthCfg) ReadBCLElement(block *bcl.Element) error {
	block.CheckEntriesOneOf("token", "token_file")

	for _, entry := range block.FindEntries("token") {
		var token string
		entry.Values(&token)
		cfg.Tokens = append(cfg.Tokens, token)
	}

	block.MaybeEntryValues("token_file", &cfg.TokenFile)

	return nil
}

type BearerAuth struct {
	Cfg    *AuthCfg
	Tokens map[string]struct{}
}

func (a *BearerAuth) Init(cfg *AuthCfg) error {
	a.Cfg = cfg

	bearerCfg := a.Cfg.Bearer

	if filePath := bearerCfg.TokenFile; filePath == "" {
		a.Tokens = make(map[string]struct{})
		for _, token := range bearerCfg.Tokens {
			a.Tokens[token] = struct{}{}
		}
	} else {
		if err := a.loadCredentials(filePath); err != nil {
			return fmt.Errorf("cannot load credentials: %w", err)
		}
	}

	return nil
}

func (a *BearerAuth) AuthenticateRequest(ctx *RequestContext) error {
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

	if strings.ToLower(scheme) != "bearer" {
		a.setWWWAuthenticate(ctx)
		err := fmt.Errorf("invalid authorization scheme %q", scheme)
		ctx.ReplyError2(401, "%v", err)
		return err
	}

	token := transformAuthSecret(authorization[space+1:], a.Cfg)

	if _, found := a.Tokens[token]; !found {
		err := errors.New("invalid token")
		ctx.ReplyError2(403, "%v", err)
		return err
	}

	return nil
}

func (a *BearerAuth) loadCredentials(filePath string) error {
	tokens, err := loadAuthSecretFile(filePath)
	if err != nil {
		return err
	}

	a.Tokens = tokens
	return nil
}

func (a *BearerAuth) setWWWAuthenticate(ctx *RequestContext) {
	value := "Bearer"

	if realm := a.Cfg.Realm; realm != "" {
		value += fmt.Sprintf(" realm=%q", realm)
	}

	header := ctx.ResponseWriter.Header()
	header.Set("WWW-Authenticate", value)
}
