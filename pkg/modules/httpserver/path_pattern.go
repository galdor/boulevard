package httpserver

import (
	"bytes"
	"fmt"
	"strings"
)

type PathPattern struct {
	Segments []PathPatternSegment
	Relative bool
	Prefix   bool
}

type PathPatternSegment struct {
	Value string // empty if "*" wildcard
}

func (pp *PathPattern) String() string {
	if len(pp.Segments) == 0 {
		return "/"
	}

	var buf bytes.Buffer

	for i, s := range pp.Segments {
		if i > 0 || !pp.Relative {
			buf.WriteByte('/')
		}

		if s.Value == "" {
			buf.WriteByte('*')
		} else {
			buf.WriteString(s.Value)
		}
	}

	if pp.Prefix {
		buf.WriteByte('/')
	}

	return buf.String()
}

func (pp *PathPattern) Parse(s string) error {
	if s == "" {
		return fmt.Errorf("empty pattern")
	}

	var segmentStrings []string

	if s[0] == '/' {
		pp.Relative = false
		segmentStrings = strings.Split(s[1:], "/")
	} else {
		pp.Relative = true
		segmentStrings = strings.Split(s, "/")
	}

	pp.Segments = make([]PathPatternSegment, len(segmentStrings))
	pp.Prefix = false

	for i, segmentString := range segmentStrings {
		if segmentString == "" {
			if i == len(segmentStrings)-1 {
				pp.Prefix = true
				pp.Segments = pp.Segments[:len(pp.Segments)-1]
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

		pp.Segments[i] = segment
	}

	return nil
}

func (pp *PathPattern) Match(path string) (bool, string) {
	var pathSegments []string

	path = strings.Trim(path, "/")
	if len(path) > 0 {
		pathSegments = strings.Split(path, "/")
	}

	for _, patternSegment := range pp.Segments {
		if len(pathSegments) == 0 {
			return false, ""
		}

		pathSegment := pathSegments[0]
		pathSegments = pathSegments[1:]

		if s := patternSegment.Value; s != "" && s != pathSegment {
			return false, ""
		}
	}

	if len(pathSegments) > 0 && !pp.Prefix {
		return false, ""
	}

	return true, strings.Join(pathSegments, "/")
}
