package httpserver

import (
	"encoding/json"
	"fmt"

	"go.n16f.net/ejson"
)

type HandlerCfg struct {
	Match MatchCfg `json:"match"`
	Auth  *AuthCfg `json:"authentication,omitempty"`

	Reply        *ReplyActionCfg        `json:"reply,omitempty"`
	Serve        *ServeActionCfg        `json:"serve,omitempty"`
	ReverseProxy *ReverseProxyActionCfg `json:"reverse_proxy,omitempty"`
	Status       *StatusActionCfg       `json:"status,omitempty"`
	FastCGI      *FastCGIActionCfg      `json:"fastcgi,omitempty"`
}

func (cfg *HandlerCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckObject("match", &cfg.Match)
	v.CheckOptionalObject("authentication", cfg.Auth)

	nbActions := 0
	if cfg.Serve != nil {
		nbActions++
	}
	if cfg.Reply != nil {
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
		v.AddError(nil, "missing_action", "handler must contain an action")
	} else if nbActions > 1 {
		v.AddError(nil, "multiple_actions",
			"handler must contain a single action")
	}

	v.CheckOptionalObject("serve", cfg.Serve)
	v.CheckOptionalObject("reply", cfg.Reply)
	v.CheckOptionalObject("reverse_proxy", cfg.ReverseProxy)
	v.CheckOptionalObject("status", cfg.Status)
	v.CheckOptionalObject("fastcgi", cfg.FastCGI)
}

type MatchCfg struct {
	Method      string       `json:"method,omitempty"`
	Path        string       `json:"path,omitempty"`
	PathPattern *PathPattern `json:"-"`
}

func (cfg *MatchCfg) MarshalJSON() ([]byte, error) {
	type MatchCfg2 MatchCfg
	cfg2 := MatchCfg2(*cfg)

	if cfg2.PathPattern != nil {
		cfg2.Path = cfg2.PathPattern.String()
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

		cfg2.PathPattern = &pp
	}

	*cfg = MatchCfg(cfg2)
	return nil
}

type Handler struct {
	Module *Module
	Cfg    HandlerCfg

	Auth   Auth
	Action Action
}

func NewHandler(mod *Module, cfg HandlerCfg) (*Handler, error) {
	h := Handler{
		Module: mod,
		Cfg:    cfg,
	}

	if authCfg := cfg.Auth; authCfg != nil {
		auth, err := NewAuth(authCfg)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize authentication: %w", err)
		}

		h.Auth = auth
	}

	var action Action
	var err error

	switch {
	case cfg.Reply != nil:
		action, err = NewReplyAction(&h, *cfg.Reply)
	case cfg.Serve != nil:
		action, err = NewServeAction(&h, *cfg.Serve)
	case cfg.ReverseProxy != nil:
		action, err = NewReverseProxyAction(&h, *cfg.ReverseProxy)
	case cfg.Status != nil:
		action, err = NewStatusAction(&h, *cfg.Status)
	case cfg.FastCGI != nil:
		action, err = NewFastCGIAction(&h, *cfg.FastCGI)
	default:
		return nil, fmt.Errorf("missing action configuration")
	}

	if err != nil {
		return nil, fmt.Errorf("cannot create action: %w", err)
	}

	h.Action = action

	return &h, nil
}

func (h *Handler) Start() error {
	return h.Action.Start()
}

func (h *Handler) Stop() {
	h.Action.Stop()
}

func (h *Handler) matchRequest(ctx *RequestContext) bool {
	matchSpec := h.Cfg.Match
	if matchSpec.Method != "" && matchSpec.Method != ctx.Request.Method {
		return false
	}

	if pattern := matchSpec.PathPattern; pattern != nil {
		match, subpath := pattern.Match(ctx.Request.URL.Path)
		if !match {
			return false
		}

		ctx.Subpath = subpath
	}

	if h.Auth != nil {
		ctx.Auth = h.Auth
	}

	return true
}
