package loadbalancer

type Server struct {
	Address string
}

func NewServer(mod *Module, address string) (*Server, error) {
	s := Server{
		Address: address,
	}

	return &s, nil
}

func (s *Server) Stop() {
}
