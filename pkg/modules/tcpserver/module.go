package tcpserver

import (
	"fmt"

	"go.n16f.net/acme"
	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/netutils"
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
	Listeners []*netutils.TCPListenerCfg

	ReadBufferSize  int
	WriteBufferSize int

	ReverseProxy ReverseProxyAction
}

func (cfg *ModuleCfg) ReadBCLElement(block *bcl.Element) error {
	block.Blocks("listener", &cfg.Listeners)
	if len(cfg.Listeners) == 0 {
		block.AddSimpleValidationError("TCP server does not contain " +
			"any listener")
	}

	cfg.ReadBufferSize = DefaultReadBufferSize
	block.MaybeEntryValue("read_buffer_size",
		bcl.WithValueValidation(&cfg.ReadBufferSize,
			bcl.ValidatePositiveInteger))

	cfg.WriteBufferSize = DefaultWriteBufferSize
	block.MaybeEntryValue("write_buffer_size",
		bcl.WithValueValidation(&cfg.WriteBufferSize,
			bcl.ValidatePositiveInteger))

	block.Element("reverse_proxy", &cfg.ReverseProxy)

	return nil
}

type ReverseProxyAction struct {
	Address string
}

func (cfg *ReverseProxyAction) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
		elt.EntryValue("address",
			bcl.WithValueValidation(&cfg.Address, netutils.ValidateBCLAddress))
	} else {
		elt.Value(
			bcl.WithValueValidation(&cfg.Address, netutils.ValidateBCLAddress))
	}

	return nil
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
