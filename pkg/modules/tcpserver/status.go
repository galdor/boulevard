package tcpserver

type Status struct {
	Listeners []*ListenerStatus `json:"listeners"`
}

type ListenerStatus struct {
	Address    string
	TLSDomains []string
}

func (mod *Module) StatusData() any {
	listeners := make([]*ListenerStatus, len(mod.listeners))

	for i, l := range mod.listeners {
		status := ListenerStatus{
			Address: l.TCPListener.Cfg.Address,
		}

		if tls := l.TCPListener.Cfg.TLS; tls != nil {
			status.TLSDomains = tls.Domains
		}

		listeners[i] = &status
	}

	return Status{
		Listeners: listeners,
	}
}
