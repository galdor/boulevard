package boulevard

import "go.n16f.net/bcl"

type ProtocolInfo struct {
	Name           string
	InstantiateCfg func() ProtocolCfg
	Instantiate    func() Protocol
}

type ProtocolCfg interface {
	bcl.ElementReader
}

type Protocol interface {
	Start(*Server) error
	Stop()

	RotateLogFiles()
	StatusData() any
}
