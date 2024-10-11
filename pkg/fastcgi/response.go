package fastcgi

import (
	"net/http"
	"strconv"
	"strings"
)

var nonHTTPHeaderFields = map[string]struct{}{
	"status": struct{}{},
}

type Response struct {
	RequestId uint16
	Header    Header
	Events    <-chan *ResponseEvent
}

func (r *Response) Status() (int, string) {
	s := r.Header.Field("Status")
	if s == "" {
		// RFC 3875 6.3.3. Status: "Status code 200 'OK' indicates success, and
		// is the default value assumed for a document response."
		return 200, "OK"
	}

	codeString, reason, _ := strings.Cut(s, " ")

	i64, err := strconv.ParseInt(codeString, 10, 64)
	if err != nil || i64 < 1 || i64 > 999 {
		return 200, "OK"
	}

	return int(i64), reason
}

func (r *Response) CopyHeaderToHTTPHeader(hh http.Header) {
	for _, field := range r.Header.Fields {
		name := strings.ToLower(field.Name)
		if _, found := nonHTTPHeaderFields[name]; found {
			continue
		}

		hh.Add(field.Name, field.Value)
	}
}
