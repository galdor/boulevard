package boulevard

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/boulevard/pkg/netutils"
)

type HealthProbeState string

const (
	HealthProbeStateSuccessful HealthProbeState = "successful"
	HealthProbeStateFailing    HealthProbeState = "failing"
	HealthProbeStateFailed     HealthProbeState = "failed"
	HealthProbeStateRecovering HealthProbeState = "recovering"
)

type HealthProbeCfg struct {
	Period           int // seconds
	SuccessThreshold int
	FailureThreshold int

	TCP  *TCPHealthTestCfg
	HTTP *HTTPHealthTestCfg
}

func (p *HealthProbeCfg) ReadBCLElement(elt *bcl.Element) error {
	p.Period = 5
	elt.MaybeEntryValues("period", &p.Period)

	p.SuccessThreshold = 1
	elt.MaybeEntryValues("success_threshold", &p.SuccessThreshold)
	p.FailureThreshold = 1
	elt.MaybeEntryValues("failure_threshold", &p.FailureThreshold)

	elt.MaybeElement("tcp", &p.TCP)
	elt.MaybeBlock("http", &p.HTTP)

	return nil
}

type HealthTest interface {
	Execute(string) error
}

type HealthProbe struct {
	Cfg *HealthProbeCfg

	address string
	tests   []HealthTest

	state HealthProbeState
	count int
}

func NewHealthProbe(address string, cfg *HealthProbeCfg) *HealthProbe {
	probe := HealthProbe{
		Cfg: cfg,

		state:   HealthProbeStateSuccessful,
		address: address,
	}

	if cfg.TCP != nil {
		probe.tests = append(probe.tests, NewTCPHealthTest(cfg.TCP))
	}

	if cfg.HTTP != nil {
		probe.tests = append(probe.tests, NewHTTPHealthTest(cfg.HTTP))
	}

	return &probe
}

func (p *HealthProbe) Execute() (bool, error) {
	successful := true
	var err error

	for _, test := range p.tests {
		if err = test.Execute(p.address); err != nil {
			successful = false
			break
		}
	}

	switch p.state {
	case HealthProbeStateSuccessful:
		if !successful {
			p.state = HealthProbeStateFailing
			p.count = 1
		}

	case HealthProbeStateFailing:
		if successful {
			p.state = HealthProbeStateSuccessful
			p.count = 0
		} else {
			p.count++
			if p.count >= p.Cfg.FailureThreshold {
				p.state = HealthProbeStateFailed
			}
		}

	case HealthProbeStateFailed:
		if successful {
			p.state = HealthProbeStateRecovering
			p.count = 1
		}

	case HealthProbeStateRecovering:
		if successful {
			p.count++
			if p.count >= p.Cfg.SuccessThreshold {
				p.state = HealthProbeStateSuccessful
			}
		} else {
			p.state = HealthProbeStateFailed
			p.count = 0
		}
	}

	ok := p.state == HealthProbeStateSuccessful ||
		p.state == HealthProbeStateFailing
	return ok, err
}

type TCPHealthTestCfg struct {
}

func (t *TCPHealthTestCfg) ReadBCLElement(elt *bcl.Element) error {
	return nil
}

type TCPHealthTest struct {
	Cfg *TCPHealthTestCfg
}

func NewTCPHealthTest(cfg *TCPHealthTestCfg) *TCPHealthTest {
	return &TCPHealthTest{
		Cfg: cfg,
	}
}

func (t *TCPHealthTest) Execute(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		err = netutils.UnwrapOpError(err, "dial")
		return fmt.Errorf("cannot connect to %s: %w", address, err)
	}
	conn.Close()
	return nil
}

type HTTPHealthTestCfg struct {
	Method string
	Path   string

	SuccessStatus int
}

func (t *HTTPHealthTestCfg) ReadBCLElement(elt *bcl.Element) error {
	t.Method = "GET"
	elt.MaybeEntryValues("method",
		bcl.WithValueValidation(&t.Method, httputils.ValidateBCLMethod))

	elt.EntryValues("path", &t.Path)

	if successBlock := elt.FindBlock("success"); successBlock != nil {
		successBlock.MaybeEntryValues("status",
			bcl.WithValueValidation(&t.SuccessStatus,
				httputils.ValidateBCLStatus))
	} else {
		t.SuccessStatus = 200
	}

	return nil
}

type HTTPHealthTest struct {
	Cfg *HTTPHealthTestCfg

	httpClient *http.Client
}

func NewHTTPHealthTest(cfg *HTTPHealthTestCfg) *HTTPHealthTest {
	transport := http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	client := http.Client{
		Timeout:   time.Duration(10 * time.Second),
		Transport: &transport,
	}

	return &HTTPHealthTest{
		Cfg: cfg,

		httpClient: &client,
	}
}

func (t *HTTPHealthTest) Execute(address string) error {
	uri := url.URL{Scheme: "http", Host: address, Path: t.Cfg.Path}
	req, err := http.NewRequest(t.Cfg.Method, uri.String(), nil)
	if err != nil {
		return fmt.Errorf("cannot create HTTP request: %w", err)
	}

	res, err := t.httpClient.Do(req)
	if err != nil {
		err = httputils.UnwrapUrlError(err)
		return fmt.Errorf("cannot send HTTP request: %w", err)
	}
	res.Body.Close()

	if status := res.StatusCode; status != t.Cfg.SuccessStatus {
		return fmt.Errorf("HTTP request failed with status %d "+
			"(expected status %d)", status, t.Cfg.SuccessStatus)
	}

	return nil
}

func ValidateBCLPortNumber(v any) error {
	port := v.(int)
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number")
	}

	return nil
}
