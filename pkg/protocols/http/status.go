package http

type Status struct {
}

func (p *Protocol) StatusData() any {
	return &Status{}
}
