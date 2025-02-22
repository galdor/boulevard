package tcp

type Status struct {
	NbConnections int `json:"nb_connections"`
}

func (p *Protocol) StatusData() any {
	var status Status

	p.connectionMutex.Lock()
	status.NbConnections = len(p.connections)
	p.connectionMutex.Unlock()

	return &status
}
