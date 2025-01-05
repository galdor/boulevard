package httpserver

import (
	"fmt"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/boulevard/pkg/utils"
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

func (cfg *HandlerCfg) Init(block *bcl.Element) {
	if elt := block.Element("match"); elt != nil {
		cfg.Match.Init(elt)
	}

	if block := block.MaybeBlock("authentication"); block != nil {
		cfg.Auth = new(AuthCfg)
		cfg.Auth.Init(block)
	}

	if block := block.MaybeBlock("access_logs"); block != nil {
		cfg.AccessLogger = new(AccessLoggerCfg)
		cfg.AccessLogger.Init(block)
	}

	for _, block := range block.Blocks("handler") {
		var hcfg HandlerCfg
		hcfg.Init(block)

		cfg.Handlers = append(cfg.Handlers, &hcfg)
	}

	block.CheckElementsOneOf("reply", "redirect", "serve", "reverse_proxy",
		"status", "fastcgi")

	if elt := block.MaybeElement("reply"); elt != nil {
		cfg.Reply = new(ReplyActionCfg)
		cfg.Reply.Init(elt)
	}

	if elt := block.MaybeElement("redirect"); elt != nil {
		cfg.Redirect = new(RedirectActionCfg)
		cfg.Redirect.Init(elt)
	}

	if elt := block.MaybeElement("serve"); elt != nil {
		cfg.Serve = new(ServeActionCfg)
		cfg.Serve.Init(elt)
	}

	if elt := block.MaybeElement("reverse_proxy"); elt != nil {
		cfg.ReverseProxy = new(ReverseProxyActionCfg)
		cfg.ReverseProxy.Init(elt)
	}

	if elt := block.MaybeElement("status"); elt != nil {
		cfg.Status = new(StatusActionCfg)
		cfg.Status.Init(elt)
	}

	if block := block.MaybeBlock("fastcgi"); block != nil {
		cfg.FastCGI = new(FastCGIActionCfg)
		cfg.FastCGI.Init(block)
	}
}

type MatchCfg struct {
	Method     string
	Host       *netutils.DomainNamePattern
	HostRegexp *utils.Regexp
	Path       *PathPattern
	PathRegexp *utils.Regexp
}

func (cfg *MatchCfg) Init(elt *bcl.Element) {
	if elt.IsBlock() {
		// TODO Validate HTTP method
		elt.MaybeEntryValue("method", &cfg.Method)
		elt.MaybeEntryValue("host", &cfg.Host)
		elt.MaybeEntryValue("host_regexp", &cfg.HostRegexp)
		elt.MaybeEntryValue("path", &cfg.Path)
		elt.MaybeEntryValue("path_regexp", &cfg.PathRegexp)
	} else {
		elt.Value(&cfg.Path)
	}
}

type Handler struct {
	Module *Module
	Cfg    *HandlerCfg

	AccessLogger *AccessLogger
	Auth         Auth
	Action       Action

	Handlers []*Handler
}

func NewHandler(mod *Module, cfg *HandlerCfg) (*Handler, error) {
	h := Handler{
		Module: mod,
		Cfg:    cfg,
	}

	if logCfg := cfg.AccessLogger; logCfg != nil {
		log, err := NewAccessLogger(logCfg, mod.Vars)
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
		if len(cfg.Handlers) == 0 {
			return nil, fmt.Errorf("missing action configuration")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("cannot create action: %w", err)
	}

	h.Handlers = make([]*Handler, len(cfg.Handlers))
	for i, cfg2 := range cfg.Handlers {
		h2, err := NewHandler(mod, cfg2)
		if err != nil {
			return nil, fmt.Errorf("cannot create subhandler: %w", err)
		}

		h.Handlers[i] = h2
	}

	h.Action = action

	return &h, nil
}

func (h *Handler) Start() error {
	if h.Action != nil {
		if err := h.Action.Start(); err != nil {
			return err
		}
	}

	for i, h2 := range h.Handlers {
		if err := h2.Start(); err != nil {
			for j := range i {
				h.Handlers[j].Stop()
			}

			return err
		}
	}

	return nil
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
