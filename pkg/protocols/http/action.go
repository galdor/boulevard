package http

type Action interface {
	Start() error
	Stop()

	HandleRequest(*RequestContext)
}
