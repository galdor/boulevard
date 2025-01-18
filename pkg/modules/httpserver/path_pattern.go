package httpserver

import (
	"bytes"
	"fmt"
	"strings"

	"go.n16f.net/bcl"
)

type PathPattern struct {
	Segments []PathPatternSegment
	Relative bool
	Prefix   bool
}

type PathPatternSegment struct {
	Value string // empty if "*" wildcard
}

func (p *PathPattern) String() string {
	if len(p.Segments) == 0 {
		return "/"
	}

	var buf bytes.Buffer

	for i, s := range p.Segments {
		if i > 0 || !p.Relative {
			buf.WriteByte('/')
		}

		if s.Value == "" {
			buf.WriteByte('*')
		} else {
			buf.WriteString(s.Value)
		}
	}

	if p.Prefix {
		buf.WriteByte('/')
	}

	return buf.String()
}

func (p *PathPattern) Parse(s string) error {
	if s == "" {
		return fmt.Errorf("empty pattern")
	}

	var segmentStrings []string

	if s[0] == '/' {
		p.Relative = false
		segmentStrings = strings.Split(s[1:], "/")
	} else {
		p.Relative = true
		segmentStrings = strings.Split(s, "/")
	}

	p.Segments = make([]PathPatternSegment, len(segmentStrings))
	p.Prefix = false

	for i, segmentString := range segmentStrings {
		if segmentString == "" {
			if i == len(segmentStrings)-1 {
				p.Prefix = true
				p.Segments = p.Segments[:len(p.Segments)-1]
				break
			}

			return fmt.Errorf("invalid empty segment")
		}

		var escaped bool
		if segmentString[0] == '\\' {
			escaped = true
			segmentString = segmentString[1:]
		}

		var segment PathPatternSegment

		if segmentString == "*" && !escaped {
		} else {
			segment.Value = segmentString
		}

		p.Segments[i] = segment
	}

	return nil
}

// bcl.ValueReader
func (p *PathPattern) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(string)
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	if err := p.Parse(s); err != nil {
		return fmt.Errorf("invalid path pattern: %w", err)
	}

	return nil
}

func (p *PathPattern) Match(path string) (bool, string) {
	var pathSegments []string

	path = strings.Trim(path, "/")
	if len(path) > 0 {
		pathSegments = strings.Split(path, "/")
	}

	for _, patternSegment := range p.Segments {
		if len(pathSegments) == 0 {
			return false, ""
		}

		pathSegment := pathSegments[0]
		pathSegments = pathSegments[1:]

		if s := patternSegment.Value; s != "" && s != pathSegment {
			return false, ""
		}
	}

	if len(pathSegments) > 0 && !p.Prefix {
		return false, ""
	}

	return true, strings.Join(pathSegments, "/")
}
