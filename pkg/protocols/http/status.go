package http

type Status struct {
	NbConnections int64 `json:"nb_connections"`
}

func (p *Protocol) StatusData() any {
	var status Status

	status.NbConnections = p.nbConnections.Load()

	return &status
}
