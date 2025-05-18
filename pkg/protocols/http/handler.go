package http

import (
	"fmt"
	"regexp"
	"slices"

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
	for _, entry := range block.FindEntries("match") {
		cfg.Match.ReadBCLEntry(entry)
	}

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
	TLS           *bool
	HTTPVersions  []HTTPVersion
	Methods       []string
	Hosts         []*netutils.DomainNamePattern
	HostRegexps   []*regexp.Regexp
	HeaderValues  map[string][]string
	HeaderRegexps map[string][]*regexp.Regexp
	Paths         []*PathPattern
	PathRegexps   []*regexp.Regexp
}

func (cfg *MatchCfg) ReadBCLEntry(entry *bcl.Element) {
	entry.CheckValueOneOf(0, "tls", "http_version", "method", "host", "header",
		"path")

	var matchType string
	if !entry.Value(0, &matchType) {
		return
	}

	switch matchType {
	case "tls":
		entry.Values(&matchType, &cfg.TLS)

	case "http_version":
		for i := 1; i < entry.NbValues(); i++ {
			if entry.CheckValueOneOf(i, HTTPVersionStringsAny...) {
				var version HTTPVersion
				entry.Value(i, &version)
				cfg.HTTPVersions = append(cfg.HTTPVersions, version)
			}
		}

	case "method":
		cfg.Methods = make([]string, entry.NbValues()-1)
		for i := 1; i < entry.NbValues(); i++ {
			entry.Value(i, bcl.WithValueValidation(&cfg.Methods[i-1],
				httputils.ValidateBCLMethod))
		}

	case "host":
		for i := 1; i < entry.NbValues(); i++ {
			var s bcl.String

			if entry.Value(i, &s) {
				switch s.Sigil {
				case "re":
					var re *regexp.Regexp
					entry.Value(i, &re)
					cfg.HostRegexps = append(cfg.HostRegexps, re)

				default:
					var pattern *netutils.DomainNamePattern
					entry.Value(i, &pattern)
					cfg.Hosts = append(cfg.Hosts, pattern)
				}
			}
		}

	case "header":
		if cfg.HeaderValues == nil {
			cfg.HeaderValues = make(map[string][]string)
		}
		if cfg.HeaderRegexps == nil {
			cfg.HeaderRegexps = make(map[string][]*regexp.Regexp)
		}

		var name string

		if entry.Value(1, &name) {
			if entry.NbValues() == 1 {
				cfg.HeaderValues[name] = nil
			}

			for i := 2; i < entry.NbValues(); i++ {
				var s bcl.String

				if entry.Value(i, &s) {
					switch s.Sigil {
					case "re":
						var re *regexp.Regexp
						entry.Value(i, &re)
						cfg.HeaderRegexps[name] =
							append(cfg.HeaderRegexps[name], re)

					default:
						var value string
						entry.Value(i, &value)
						cfg.HeaderValues[name] =
							append(cfg.HeaderValues[name], value)
					}
				}
			}
		}

	case "path":
		for i := 1; i < entry.NbValues(); i++ {
			var s bcl.String

			if entry.Value(i, &s) {
				switch s.Sigil {
				case "re":
					var re *regexp.Regexp
					entry.Value(i, &re)
					cfg.PathRegexps = append(cfg.PathRegexps, re)

				default:
					var pattern *PathPattern
					entry.Value(i, &pattern)
					cfg.Paths = append(cfg.Paths, pattern)
				}
			}
		}
	}
}

func (cfg *MatchCfg) HasPaths() bool {
	return len(cfg.Paths) > 0 || len(cfg.PathRegexps) > 0

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
	// A request matches if all constraints match.
	//
	// Careful, we only update the request context if we have a full match. This
	// is important because we try to match handlers recursively and fall back
	// to the last parent handler which matched.

	matchSpec := h.Cfg.Match

	// TLS
	if matchSpec.TLS != nil {
		if *matchSpec.TLS == true && ctx.Request.TLS == nil {
			return false
		}

		if *matchSpec.TLS == false && ctx.Request.TLS != nil {
			return false
		}
	}

	// HTTP version
	if len(matchSpec.HTTPVersions) > 0 {
		var versionMatch bool

		for _, version := range matchSpec.HTTPVersions {
			if version.Match(ctx.Request.ProtoMajor, ctx.Request.ProtoMinor) {
				versionMatch = true
				break
			}
		}

		if !versionMatch {
			return false
		}
	}

	// Method
	if len(matchSpec.Methods) > 0 {
		if !slices.Contains(matchSpec.Methods, ctx.Request.Method) {
			return false
		}
	}

	// Host
	if len(matchSpec.Hosts) > 0 || len(matchSpec.HostRegexps) > 0 {
		var hostMatch bool

		if patterns := matchSpec.Hosts; len(patterns) > 0 {
			for _, pattern := range patterns {
				if pattern.Match(ctx.Host) {
					hostMatch = true
					break
				}
			}
		}

		if !hostMatch {
			if res := matchSpec.HostRegexps; len(res) > 0 {
				for _, re := range res {
					if re.MatchString(ctx.Host) {
						hostMatch = true
						break
					}
				}
			}
		}

		if !hostMatch {
			return false
		}
	}

	// Header
	if len(matchSpec.HeaderValues) > 0 || len(matchSpec.HeaderRegexps) > 0 {
		header := ctx.Request.Header
		var headerMatch bool

	outer1:
		for name, expectedValues := range matchSpec.HeaderValues {
			if len(expectedValues) == 0 {
				headerMatch = len(header.Values(name)) > 0
			} else {
				for _, value := range header.Values(name) {
					if slices.Contains(expectedValues, value) {
						headerMatch = true
						break outer1
					}
				}
			}
		}

		if !headerMatch {
		outer2:
			for name, res := range matchSpec.HeaderRegexps {
				for _, value := range header.Values(name) {
					for _, re := range res {
						if re.MatchString(value) {
							headerMatch = true
							break outer2
						}
					}
				}
			}
		}

		if !headerMatch {
			return false
		}
	}

	// Path
	var subpath string

	if matchSpec.HasPaths() {
		var pathMatch bool

		if patterns := matchSpec.Paths; len(patterns) > 0 {
			for _, pattern := range patterns {
				refPath := ctx.Request.URL.Path
				if pattern.Relative {
					refPath = ctx.Subpath
				}

				var pMatch bool
				pMatch, subpath = pattern.Match(refPath)
				if pMatch {
					pathMatch = true
					break
				}
			}
		}

		if !pathMatch {
			if res := matchSpec.PathRegexps; len(res) > 0 {
				for _, re := range res {
					if re.MatchString(ctx.Request.URL.Path) {
						pathMatch = true
						break
					}
				}
			}
		}

		if !pathMatch {
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
