package httputils

import "net/http"

func Header(fields ...string) http.Header {
	header := make(http.Header)

	for i := 0; i < len(fields); {
		header.Add(fields[i], fields[i+1])
		i += 2
	}

	return header
}
