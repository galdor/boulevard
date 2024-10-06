package fastcgi

import (
	"bytes"
	"fmt"
	"strings"
)

// Reference: RFC 3875 6.3. Response Header Fields.

type Header struct {
	Fields []Field
	Data   []byte
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

func (h *Header) Parse(data []byte) (bool, []byte, error) {
	h.Data = append(h.Data, data...)

	for len(h.Data) > 0 {
		if data2, found := skipEOL(h.Data); found {
			h.Data = nil
			return true, data2, nil
		}

		eol := bytes.IndexByte(h.Data, '\n')
		if eol == -1 {
			return false, nil, nil
		}

		fieldData := h.Data[:eol]
		if len(fieldData) > 0 && fieldData[len(fieldData)-1] == '\r' {
			fieldData = fieldData[:len(fieldData)-1]
		}

		var field Field
		if err := field.Parse(fieldData); err != nil {
			return false, nil, fmt.Errorf("cannot parse field %q: %w",
				string(fieldData), err)
		}

		h.Fields = append(h.Fields, field)

		h.Data = h.Data[eol+1:]
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
