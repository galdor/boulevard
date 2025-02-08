package service

import (
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/protocols/http"
	"go.n16f.net/boulevard/pkg/protocols/tcp"
)

var DefaultProtocols = []*boulevard.ProtocolInfo{
	tcp.ProtocolInfo(),
	http.ProtocolInfo(),
}
