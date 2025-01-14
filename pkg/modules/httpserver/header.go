package httpserver

import (
	"net/http"

	"go.n16f.net/boulevard/pkg/boulevard"
)

type Header map[string]*boulevard.FormatString

func (h Header) Apply(header http.Header, vars map[string]string) {
	for name, value := range h {
		value := value.Expand(vars)

		if value == "" {
			header.Del(name)
		} else {
			header.Add(name, value)
		}
	}
}
