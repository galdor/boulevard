package http

import (
	"fmt"
	"regexp"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/boulevard/pkg/netutils"
)

type HandlerCfg struct {
	Match        MatchCfg
	Auth         *AuthCfg
	AccessLogger *AccessLoggerCfg

	Reply        *ReplyActionCfg
	Redirect     *RedirectActionCfg
	Serve        *ServeActionCfg
	ReverseProxy *ReverseProxyActionCfg
	Status       *StatusActionCfg
	FastCGI      *FastCGIActionCfg

	Handlers []*HandlerCfg
}

func (cfg *HandlerCfg) ReadBCLElement(block *bcl.Element) error {
	block.Element("match", &cfg.Match)

	block.MaybeBlock("authentication", &cfg.Auth)
	block.MaybeBlock("access_logs", &cfg.AccessLogger)

	block.CheckElementsMaybeOneOf("reply", "redirect", "serve", "reverse_proxy",
		"status", "fastcgi")
	block.MaybeElement("reply", &cfg.Reply)
	block.MaybeElement("redirect", &cfg.Redirect)
	block.MaybeElement("serve", &cfg.Serve)
	block.MaybeElement("reverse_proxy", &cfg.ReverseProxy)
	block.MaybeElement("status", &cfg.Status)
	block.MaybeElement("fastcgi", &cfg.FastCGI)

	block.Blocks("handler", &cfg.Handlers)

	return nil
}

type MatchCfg struct {
	Method     string
	Host       *netutils.DomainNamePattern
	HostRegexp *regexp.Regexp
	Path       *PathPattern
	PathRegexp *regexp.Regexp
}

func (cfg *MatchCfg) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
		elt.MaybeEntryValue("method",
			bcl.WithValueValidation(&cfg.Method, httputils.ValidateBCLMethod))
		elt.MaybeEntryValue("host", &cfg.Host)
		elt.MaybeEntryValue("host_regexp", &cfg.HostRegexp)
		elt.MaybeEntryValue("path", &cfg.Path)
		elt.MaybeEntryValue("path_regexp", &cfg.PathRegexp)
	} else {
		elt.Value(&cfg.Path)
	}

	return nil
}

type Handler struct {
	Protocol *Protocol
	Cfg      *HandlerCfg

	AccessLogger *AccessLogger
	Auth         Auth
	Action       Action

	Handlers []*Handler
}

func StartHandler(p *Protocol, cfg *HandlerCfg) (*Handler, error) {
	h := Handler{
		Protocol: p,
		Cfg:      cfg,
	}

	if logCfg := cfg.AccessLogger; logCfg != nil {
		log, err := NewAccessLogger(logCfg, p.vars)
		if err != nil {
			return nil, fmt.Errorf("cannot create access logger: %w", err)
		}

		h.AccessLogger = log
	}

	if authCfg := cfg.Auth; authCfg != nil {
		auth, err := NewAuth(authCfg)
		if err != nil {
			if h.AccessLogger != nil {
				h.AccessLogger.Close()
			}

			return nil, fmt.Errorf("cannot initialize authentication: %w", err)
		}

		h.Auth = auth
	}

	var action Action
	var err error

	switch {
	case cfg.Reply != nil:
		action, err = NewReplyAction(&h, cfg.Reply)
	case cfg.Redirect != nil:
		action, err = NewRedirectAction(&h, cfg.Redirect)
	case cfg.Serve != nil:
		action, err = NewServeAction(&h, cfg.Serve)
	case cfg.ReverseProxy != nil:
		action, err = NewReverseProxyAction(&h, cfg.ReverseProxy)
	case cfg.Status != nil:
		action, err = NewStatusAction(&h, cfg.Status)
	case cfg.FastCGI != nil:
		action, err = NewFastCGIAction(&h, cfg.FastCGI)
	default:
		reply := ReplyActionCfg{Status: 200}
		action, err = NewReplyAction(&h, &reply)
	}

	if err != nil {
		return nil, fmt.Errorf("cannot create action: %w", err)
	}

	h.Action = action
	if err := h.Action.Start(); err != nil {
		return nil, fmt.Errorf("cannot start action: %w", err)
	}

	h.Handlers = make([]*Handler, len(cfg.Handlers))
	for i, cfg2 := range cfg.Handlers {
		h2, err := StartHandler(p, cfg2)
		if err != nil {
			for j := range i {
				h.Handlers[j].Stop()
			}

			return nil, err
		}

		h.Handlers[i] = h2
	}

	return &h, nil
}

func (h *Handler) Stop() {
	for _, h2 := range h.Handlers {
		h2.Stop()
	}

	if h.Action != nil {
		h.Action.Stop()
	}

	if h.AccessLogger != nil {
		h.AccessLogger.Close()
	}
}

func (h *Handler) matchRequest(ctx *RequestContext) bool {
	// Careful here, we only update the request context if we have a full match.
	// This is important because we try to match handlers recursively and fall
	// back to the last parent handler which matched.

	matchSpec := h.Cfg.Match
	if matchSpec.Method != "" && matchSpec.Method != ctx.Request.Method {
		return false
	}

	// Host
	if pattern := matchSpec.Host; pattern != nil {
		if !pattern.Match(ctx.Host) {
			return false
		}
	}

	if re := matchSpec.HostRegexp; re != nil {
		if !re.MatchString(ctx.Host) {
			return false
		}
	}

	// Path
	var subpath string
	if pattern := matchSpec.Path; pattern != nil {
		refPath := ctx.Request.URL.Path
		if pattern.Relative {
			refPath = ctx.Subpath
		}

		var match bool
		match, subpath = pattern.Match(refPath)
		if !match {
			return false
		}
	}

	if re := matchSpec.PathRegexp; re != nil {
		if !re.MatchString(ctx.Request.URL.Path) {
			return false
		}
	}

	// We now have a full match, we can update the request context
	ctx.Subpath = subpath
	ctx.Vars["http.match.subpath"] = subpath

	if h.Auth != nil {
		ctx.Auth = h.Auth
	}

	if h.AccessLogger != nil {
		ctx.AccessLogger = h.AccessLogger
	}

	return true
}
