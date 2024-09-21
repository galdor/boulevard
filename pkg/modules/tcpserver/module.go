package tcpserver

import (
	"fmt"

	"go.n16f.net/acme"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

const (
	DefaultReadBufferSize  = 16 * 1024
	DefaultWriteBufferSize = 16 * 1024
)

func ModuleInfo() *boulevard.ModuleInfo {
	return &boulevard.ModuleInfo{
		Type:           "tcp_server",
		InstantiateCfg: NewModuleCfg,
		Instantiate:    NewModule,
	}
}

type ModuleCfg struct {
	Listeners []*netutils.TCPListenerCfg `json:"listeners"`

	ReadBufferSize  int `json:"read_buffer_size,omitempty"`
	WriteBufferSize int `json:"write_buffer_size,omitempty"`

	ReverseProxy ReverseProxyAction `json:"reverse_proxy"`
}

func (cfg *ModuleCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckArrayNotEmpty("listeners", cfg.Listeners)
	v.CheckObjectArray("listeners", cfg.Listeners)

	if cfg.ReadBufferSize == 0 {
		cfg.ReadBufferSize = DefaultReadBufferSize
	} else {
		v.CheckIntMin("read_buffer_size", cfg.ReadBufferSize, 1)
	}

	if cfg.WriteBufferSize == 0 {
		cfg.WriteBufferSize = DefaultWriteBufferSize
	} else {
		v.CheckIntMin("write_buffer_size", cfg.WriteBufferSize, 1)
	}

	v.CheckObject("reverse_proxy", &cfg.ReverseProxy)
}

type ReverseProxyAction struct {
	Address string `json:"address"`
}

func (a *ReverseProxyAction) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("address", a.Address)
}

func NewModuleCfg() boulevard.ModuleCfg {
	return &ModuleCfg{}
}

type Module struct {
	Cfg *ModuleCfg
	Log *log.Logger

	errChan    chan<- error
	acmeClient *acme.Client
	listeners  []*Listener
}

func NewModule() boulevard.Module {
	return &Module{}
}

func (mod *Module) Start(modCfg boulevard.ModuleCfg, modData *boulevard.ModuleData) error {
	mod.Cfg = modCfg.(*ModuleCfg)
	mod.Log = modData.Logger

	mod.errChan = modData.ErrChan
	mod.acmeClient = modData.ACMEClient

	mod.listeners = make([]*Listener, len(mod.Cfg.Listeners))

	for i, cfg := range mod.Cfg.Listeners {
		if cfg.TLS != nil {
			cfg.TLS.CertificateName = fmt.Sprintf("%s-%d", modData.Name, i)
		}

		listener, err := NewListener(mod, *cfg)
		if err != nil {
			return fmt.Errorf("cannot create listener: %w", err)
		}

		if err := listener.Start(); err != nil {
			for j := range i {
				mod.listeners[j].Stop()
			}

			return fmt.Errorf("cannot start listener: %w", err)
		}

		mod.listeners[i] = listener
	}

	return nil
}

func (mod *Module) Stop() {
	for _, listener := range mod.listeners {
		listener.Stop()
	}
}
