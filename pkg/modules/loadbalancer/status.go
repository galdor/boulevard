package loadbalancer

type Status struct {
	Servers []*ServerStatus `json:"servers"`
}

type ServerStatus struct {
	Address string `json:"address"`
}

func (mod *Module) StatusData() any {
	servers := make([]*ServerStatus, len(mod.servers))

	for i, s := range mod.servers {
		status := ServerStatus{
			Address: s.Address,
		}

		servers[i] = &status
	}

	return Status{
		Servers: servers,
	}
}
