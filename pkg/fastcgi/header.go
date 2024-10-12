package fastcgi

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Reference: RFC 3875 6.3. Response Header Fields.

var nonHTTPHeaderFields = map[string]struct{}{
	"status": struct{}{},
}

type Header struct {
	Fields []Field
	data   []byte
}

type Field struct {
	Name  string
	Value string
}

func (h *Header) Field(name string) string {
	for _, field := range h.Fields {
		if strings.EqualFold(name, field.Name) {
			return field.Value
		}
	}

	return ""
}

func (h *Header) Status() (int, string) {
	s := h.Field("Status")
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

func (h *Header) CopyToHTTPHeader(hh http.Header) {
	for _, field := range h.Fields {
		name := strings.ToLower(field.Name)
		if _, found := nonHTTPHeaderFields[name]; found {
			continue
		}

		hh.Add(field.Name, field.Value)
	}
}

func (h *Header) Parse(data []byte) (bool, []byte, error) {
	h.data = append(h.data, data...)

	for len(h.data) > 0 {
		if data2, found := skipEOL(h.data); found {
			h.data = nil
			return true, data2, nil
		}

		eol := bytes.IndexByte(h.data, '\n')
		if eol == -1 {
			return false, nil, nil
		}

		fieldData := h.data[:eol]
		if len(fieldData) > 0 && fieldData[len(fieldData)-1] == '\r' {
			fieldData = fieldData[:len(fieldData)-1]
		}

		var field Field
		if err := field.Parse(fieldData); err != nil {
			return false, nil, fmt.Errorf("cannot parse field %q: %w",
				string(fieldData), err)
		}

		h.Fields = append(h.Fields, field)

		h.data = h.data[eol+1:]
	}

	return false, nil, nil
}

func (f *Field) Parse(data []byte) error {
	colon := bytes.IndexByte(data, ':')
	if colon == -1 {
		return fmt.Errorf("missing ':' separator")
	}

	name := data[:colon]
	if len(name) == 0 {
		return fmt.Errorf("empty name")
	}
	f.Name = string(name)

	value := bytes.Trim(data[colon+1:], " \t")
	f.Value = string(value)

	return nil
}

func skipEOL(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return data, false
	}

	if data[0] == '\n' {
		return data[1:], true
	}

	if data[0] == '\r' {
		if len(data) > 1 && data[1] == '\n' {
			return data[2:], true
		}
	}

	return data, false
}
