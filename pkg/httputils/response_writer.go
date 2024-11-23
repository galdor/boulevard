package httputils

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type ResponseWriter struct {
	Status   int
	BodySize int

	w http.ResponseWriter
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		w: w,
	}
}

func (w *ResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *ResponseWriter) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)
	w.BodySize += n
	return n, err
}

func (w *ResponseWriter) WriteHeader(status int) {
	w.Status = status
	w.w.WriteHeader(status)
}

func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.w.(http.Hijacker)
	if !ok {
		return nil, nil,
			fmt.Errorf("response writer does not support connection hijacking")
	}

	return hijacker.Hijack()
}

func (w *ResponseWriter) Flush() {
	f := w.w.(http.Flusher)
	f.Flush()
}
