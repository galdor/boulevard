package boulevard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"go.n16f.net/ejson"
	"go.n16f.net/program"
)

type FormatString struct {
	value   string
	parts   []FormatStringPart
	lenHint int
}

type FormatStringPartType string

const (
	FormatStringPartTypeConstant FormatStringPartType = "constant"
	FormatStringPartTypeVariable FormatStringPartType = "variable"
)

type FormatStringPart struct {
	Type  FormatStringPartType
	Value string
}

func (s *FormatString) Parse(value string) error {
	var parts []FormatStringPart
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

			part := FormatStringPart{
				Type:  FormatStringPartTypeConstant,
				Value: string(data[:2]),
			}
			parts = append(parts, part)

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
			if err := validateEnvironmentVariableName(name); err != nil {
				return fmt.Errorf("invalid environment variable name %q: %w",
					name, err)
			}

			value, found := os.LookupEnv(name)
			if !found {
				return fmt.Errorf("unknown environment variable %q", name)
			}

			part := FormatStringPart{
				Type:  FormatStringPartTypeConstant,
				Value: value,
			}
			parts = append(parts, part)

			lenHint += len(value)

			data = data[end+3:]

		case '{':
			end := bytes.IndexByte(data[1:], '}')
			if end == -1 {
				return fmt.Errorf("truncated variable block %q", string(data))
			}

			name := string(data[1 : end+1])
			if err := validateFormatStringVariableName(name); err != nil {
				return fmt.Errorf("invalid variable name %q: %w", name, err)
			}

			part := FormatStringPart{
				Type:  FormatStringPartTypeVariable,
				Value: name,
			}
			parts = append(parts, part)

			lenHint += 16

			data = data[end+2:]

		default:
			end := bytes.IndexAny(data, "\\${")
			if end == -1 {
				end = len(data)
			}

			part := FormatStringPart{
				Type:  FormatStringPartTypeConstant,
				Value: string(data[:end]),
			}
			parts = append(parts, part)

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

	*s = FormatString{value: value}
	return nil
}

func (s *FormatString) ValidateJSON(v *ejson.Validator) {
	if err := s.Parse(s.value); err != nil {
		v.AddError(nil, "invalid_format_string",
			"invalid format string: %v", err)
	}
}

func (s FormatString) Expand(vars map[string]string) string {
	var buf bytes.Buffer

	buf.Grow(s.lenHint)

	for _, part := range s.parts {
		var partString string

		switch part.Type {
		case FormatStringPartTypeConstant:
			partString = part.Value
		case FormatStringPartTypeVariable:
			partString = vars[part.Value]
		default:
			program.Panic("unknown string part type %q", part.Type)
		}

		buf.WriteString(partString)
	}

	return buf.String()
}

func validateFormatStringVariableName(name string) error {
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '_':
		case c == '.':
		default:
			return fmt.Errorf("invalid character %q", c)
		}
	}

	return nil
}

func validateEnvironmentVariableName(name string) error {
	for _, c := range name {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_':
		default:
			return fmt.Errorf("invalid character %q", c)
		}
	}

	return nil
}
