package httpserver

import (
	"encoding/json"
	"fmt"
	"regexp"

	"go.n16f.net/ejson"
)

type HandlerCfg struct {
	Match        MatchCfg         `json:"match"`
	Auth         *AuthCfg         `json:"authentication,omitempty"`
	AccessLogger *AccessLoggerCfg `json:"access_logs,omitempty"`

	Reply        *ReplyActionCfg        `json:"reply,omitempty"`
	Redirect     *RedirectActionCfg     `json:"redirect,omitempty"`
	Serve        *ServeActionCfg        `json:"serve,omitempty"`
	ReverseProxy *ReverseProxyActionCfg `json:"reverse_proxy,omitempty"`
	Status       *StatusActionCfg       `json:"status,omitempty"`
	FastCGI      *FastCGIActionCfg      `json:"fastcgi,omitempty"`

	Handlers []*HandlerCfg `json:"handlers,omitempty"`
}

func (cfg *HandlerCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckObject("match", &cfg.Match)
	v.CheckOptionalObject("authentication", cfg.Auth)
	v.CheckOptionalObject("access_logs", cfg.AccessLogger)

	nbActions := 0
	if cfg.Reply != nil {
		nbActions++
	}
	if cfg.Redirect != nil {
		nbActions++
	}
	if cfg.Serve != nil {
		nbActions++
	}
	if cfg.ReverseProxy != nil {
		nbActions++
	}
	if cfg.Status != nil {
		nbActions++
	}
	if cfg.FastCGI != nil {
		nbActions++
	}

	if nbActions == 0 {
		if len(cfg.Handlers) == 0 {
			v.AddError(nil, "missing_action",
				"handler with no subhandlers must contain an action")
		}
	} else if nbActions > 1 {
		v.AddError(nil, "multiple_actions",
			"handler must contain a single action")
	}

	v.CheckOptionalObject("reply", cfg.Reply)
	v.CheckOptionalObject("redirect", cfg.Redirect)
	v.CheckOptionalObject("serve", cfg.Serve)
	v.CheckOptionalObject("reverse_proxy", cfg.ReverseProxy)
	v.CheckOptionalObject("status", cfg.Status)
	v.CheckOptionalObject("fastcgi", cfg.FastCGI)

	v.CheckObjectArray("handlers", cfg.Handlers)
}

type MatchCfg struct {
	Method string `json:"method,omitempty"`

	Path        string `json:"path,omitempty"`
	pathPattern *PathPattern

	PathRegexp string `json:"path_regexp,omitempty"`
	pathRE     *regexp.Regexp
}

func (cfg *MatchCfg) MarshalJSON() ([]byte, error) {
	type MatchCfg2 MatchCfg
	cfg2 := MatchCfg2(*cfg)

	if cfg2.pathPattern != nil {
		cfg2.Path = cfg2.pathPattern.String()
	}

	if cfg2.pathRE != nil {
		cfg2.PathRegexp = cfg2.pathRE.String()
	}

	return json.Marshal(cfg2)
}

func (cfg *MatchCfg) UnmarshalJSON(data []byte) error {
	type MatchCfg2 MatchCfg
	cfg2 := MatchCfg2(*cfg)

	if err := json.Unmarshal(data, &cfg2); err != nil {
		return err
	}

	if cfg2.Path != "" {
		var pp PathPattern

		if err := pp.Parse(cfg2.Path); err != nil {
			return fmt.Errorf("cannot parse path pattern: %w", err)
		}

		cfg2.pathPattern = &pp
	}

	if cfg2.PathRegexp != "" {
		re, err := regexp.Compile(cfg2.PathRegexp)
		if err != nil {
			return fmt.Errorf("cannot compile regexp: %w", err)
		}

		cfg2.pathRE = re
	}

	*cfg = MatchCfg(cfg2)
	return nil
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
		log, err := mod.NewAccessLogger(logCfg)
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
		action, err = NewReplyAction(&h, *cfg.Reply)
	case cfg.Redirect != nil:
		action, err = NewRedirectAction(&h, *cfg.Redirect)
	case cfg.Serve != nil:
		action, err = NewServeAction(&h, *cfg.Serve)
	case cfg.ReverseProxy != nil:
		action, err = NewReverseProxyAction(&h, *cfg.ReverseProxy)
	case cfg.Status != nil:
		action, err = NewStatusAction(&h, *cfg.Status)
	case cfg.FastCGI != nil:
		action, err = NewFastCGIAction(&h, *cfg.FastCGI)
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

	var subpath string
	if pattern := matchSpec.pathPattern; pattern != nil {
		var match bool
		match, subpath = pattern.Match(ctx.Request.URL.Path)
		if !match {
			return false
		}
	}

	if re := matchSpec.pathRE; re != nil {
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
