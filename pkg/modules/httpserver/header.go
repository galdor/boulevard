package httpserver

import (
	"net/http"
)

type Header map[string]string

func (h Header) Apply(header http.Header) {
	for name, value := range h {
		if value == "" {
			header.Del(name)
		} else {
			header.Add(name, value)
		}
	}
}
