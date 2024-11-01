package boulevard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"go.n16f.net/ejson"
	"go.n16f.net/program"
)

// Dynamic string values are represented as lists of parts where each part
// starts with a character indicating the type of the part. For example
// "/foo/{http.request.method}/" is represented as ["S/foo/",
// "Vhttp.request.method", "S/"]. It might seem strange, but it means we can
// generate the final string value without any runtime type dispatch. It is not
// clear if it is the best way to do it, but it certainly is not the worse.

type FormatString struct {
	value   string
	parts   []string
	lenHint int
}

func (s *FormatString) Parse(value string) error {
	var parts []string
	var lenHint int

	data := []byte(value)

	for len(data) > 0 {
		switch data[0] {
		case '\\':
			if len(data) == 1 {
				return fmt.Errorf("truncated escape sequence %q", string(data))
			} else if data[1] != '$' && data[1] != '{' {
				return fmt.Errorf("invalid escape sequence %q", string(data))
			}

			parts = append(parts, "S"+string(data[:2]))
			lenHint += 2

			data = data[2:]
		case '$':
			if len(data) == 1 {
				return fmt.Errorf("truncated environment variable block %q",
					string(data))
			} else if data[1] != '{' {
				return fmt.Errorf("invalid character %q after '$', "+
					"expected '{'", data[1])
			}

			end := bytes.IndexByte(data[2:], '}')
			if end == -1 {
				return fmt.Errorf("truncated environment variable block %q",
					string(data))
			}

			name := string(data[2 : end+2])
			value, found := os.LookupEnv(name)
			if !found {
				return fmt.Errorf("unknown environment variable %q", name)
			}

			parts = append(parts, "S"+value)
			lenHint += len(value)

			data = data[end+3:]

		case '{':
			end := bytes.IndexByte(data[1:], '}')
			if end == -1 {
				return fmt.Errorf("truncated variable block %q", string(data))
			}

			parts = append(parts, "V"+string(data[1:end+1]))
			lenHint += 16

			data = data[end+2:]

		default:
			end := bytes.IndexAny(data, "\\${")
			if end == -1 {
				end = len(data)
			}

			parts = append(parts, "S"+string(data[:end]))
			lenHint += end

			data = data[end:]
		}
	}

	s.value = value
	s.parts = parts
	s.lenHint = lenHint

	return nil
}

func (s *FormatString) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

func (s *FormatString) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return s.Parse(value)
}

func CheckFormatString(v *ejson.Validator, token any, bs *FormatString, s string) bool {
	if err := bs.Parse(s); err != nil {
		v.AddError(token, "invalid_format_string",
			"invalid format string: %v", err)
		return false
	}

	return true
}

func (s FormatString) Expand(vars map[string]string) string {
	var buf bytes.Buffer

	buf.Grow(s.lenHint)

	for _, part := range s.parts {
		value := part[1:]

		var partString string

		switch part[0] {
		case 'S':
			partString = value
		case 'V':
			partString = vars[value]
		default:
			program.Panic("unknown string part type %q", part[0])
		}

		buf.WriteString(partString)
	}

	return buf.String()
}
