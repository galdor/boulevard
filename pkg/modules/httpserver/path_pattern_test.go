package httpserver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathPatternParse(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		s string
		p PathPattern
	}{
		{"/",
			PathPattern{
				Prefix: true,
			},
		},
		{"/foo",
			PathPattern{
				Segments: []PathPatternSegment{
					{Value: "foo"},
				},
			},
		},
		{"/foo/",
			PathPattern{
				Segments: []PathPatternSegment{
					{Value: "foo"},
				},
				Prefix: true,
			},
		},
		{"/foo/bar/baz",
			PathPattern{
				Segments: []PathPatternSegment{
					{Value: "foo"},
					{Value: "bar"},
					{Value: "baz"},
				},
			},
		},
		{"/foo/bar/baz/",
			PathPattern{
				Segments: []PathPatternSegment{
					{Value: "foo"},
					{Value: "bar"},
					{Value: "baz"},
				},
				Prefix: true,
			},
		},
		{"/foo/*/bar/*",
			PathPattern{
				Segments: []PathPatternSegment{
					{Value: "foo"},
					{Value: ""},
					{Value: "bar"},
					{Value: ""},
				},
			},
		},
		{"/\\*/bar",
			PathPattern{
				Segments: []PathPatternSegment{
					{Value: "*"},
					{Value: "bar"},
				},
			},
		},
	}

	for _, test := range tests {
		label := test.s

		var p PathPattern
		err := p.Parse(test.s)
		if assert.NoError(err, label) {
			assert.Equal(test.p, p, label)
		}
	}
}

func TestPathPatternMatch(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		{"/", "/", true},
		{"/foo", "/foo", true},
		{"/foo", "/foo/", true},
		{"/foo", "/bar", false},
		{"/foo/", "/foo", true},
		{"/foo/", "/foo/", true},
		{"/foo/", "/foo/bar", true},
		{"/foo/", "/foo/bar/", true},
		{"/foo/bar", "/foo", false},
		{"/foo/bar/", "/foo", false},
		{"/foo/bar", "/foo/bar", true},
		{"/foo/bar/", "/foo/bar", true},
		{"/foo/bar/", "/foo/bar/", true},
		{"/foo/bar/", "/foo/bar/baz", true},
		{"/*", "/", false},
		{"/*", "/foo", true},
		{"/*", "/foo/", true},
		{"/*/", "/foo", true},
		{"/*/", "/foo/", true},
		{"/*/", "/foo/bar", true},
		{"/*/", "/foo/bar/", true},
		{"/foo/*/baz", "/foo", false},
		{"/foo/*/baz", "/foo/bar", false},
		{"/foo/*/baz", "/foo/bar/baz", true},
		{"/foo/*/baz", "/foo/bar/baz", true},
		{"/foo/*/baz", "/foo/bar/baz/", true},
	}

	for _, test := range tests {
		label := fmt.Sprintf("pattern %q, path %q", test.pattern, test.path)

		var pattern PathPattern
		err := pattern.Parse(test.pattern)
		if assert.NoError(err, label) {
			assert.Equal(test.match, pattern.Match(test.path), label)
		}
	}
}
