package httpserver

type Action interface {
	Start() error
	Stop()

	HandleRequest(*RequestContext)
}
