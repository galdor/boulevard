package boulevard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"go.n16f.net/program"
)

// Dynamic string values are represented as lists of parts where each part
// starts with a character indicating the type of the part. For example
// "/foo/{http.request.method}/${MY_ENV_VAR}" is represented as ["S/foo/",
// "Vhttp.request.method", "S/", "EMY_ENV_VAR"]. It might seem strange, but it
// means we can generate the final string value without any runtime type
// dispatch. It is not clear if it is the best way to do it, but it certainly is
// not the worse.

type String struct {
	value   string
	parts   []string
	lenHint int
}

func (s *String) Parse(value string) error {
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

			parts = append(parts, "E"+string(data[2:end+2]))
			lenHint += 16

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

func (s *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

func (s *String) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return s.Parse(value)
}

func (s String) Expand(vars map[string]string) string {
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
		case 'E':
			partString = os.Getenv(value)
		default:
			program.Panic("unknown string part type %q", part[0])
		}

		buf.WriteString(partString)
	}

	return buf.String()
}
