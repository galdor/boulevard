package loadbalancer

type Status struct {
}

type ListenerStatus struct {
}

func (mod *Module) StatusData() any {
	return Status{}
}
