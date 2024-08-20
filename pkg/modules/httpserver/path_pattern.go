package httpserver

import (
	"bytes"
	"fmt"
	"strings"
)

type PathPattern struct {
	Segments []PathPatternSegment
	Prefix   bool
}

type PathPatternSegment struct {
	Value string // empty if "*" wildcard
}

func (pp *PathPattern) String() string {
	var buf bytes.Buffer

	if len(pp.Segments) == 0 {
		buf.WriteByte('/')
	}

	for _, s := range pp.Segments {
		buf.WriteByte('/')

		if s.Value == "" {
			buf.WriteByte('*')
		} else {
			buf.WriteString(s.Value)
		}
	}

	if pp.Prefix && len(pp.Segments) > 0 {
		buf.WriteByte('/')
	}

	return buf.String()
}

func (pp *PathPattern) Parse(s string) error {
	if s == "" {
		return fmt.Errorf("empty pattern")
	}

	if s[0] != '/' {
		return fmt.Errorf("invalid initial character %q", s[0])
	}

	if len(s) == 1 {
		pp.Segments = nil
		pp.Prefix = true
		return nil
	}

	segmentStrings := strings.Split(s[1:], "/")
	segments := make([]PathPatternSegment, len(segmentStrings))

	pp.Prefix = false

	for i, segmentString := range segmentStrings {
		if segmentString == "" {
			if i == len(segmentStrings)-1 {
				pp.Prefix = true
				segments = segments[:len(segments)-1]
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

		segments[i] = segment
	}

	pp.Segments = segments
	return nil
}

func (pp *PathPattern) Match(path string) bool {
	var pathSegments []string

	path = strings.Trim(path, "/")
	if len(path) > 0 {
		pathSegments = strings.Split(path, "/")
	}

	for _, patternSegment := range pp.Segments {
		if len(pathSegments) == 0 {
			return false
		}

		pathSegment := pathSegments[0]
		pathSegments = pathSegments[1:]

		if s := patternSegment.Value; s != "" && s != pathSegment {
			return false
		}
	}

	if len(pathSegments) > 0 && !pp.Prefix {
		return false
	}

	return true
}
