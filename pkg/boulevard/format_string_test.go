package boulevard

import (
	"testing"
)

func TestStringParse(t *testing.T) {
	t.Setenv("BOULEVARD_TEST_X", "42")

	tests := []struct {
		es   string
		s    string
		vars map[string]string
	}{
		{
			"",
			"",
			map[string]string{},
		},
		{
			"foo",
			"foo",
			map[string]string{},
		},
		{
			"foo",
			"{a}",
			map[string]string{
				"a": "foo",
			},
		},
		{
			"afooc",
			"a{b}c",
			map[string]string{
				"b": "foo",
			},
		},
		{
			"foobbar",
			"{a}b{c}",
			map[string]string{
				"a": "foo",
				"c": "bar",
			},
		},
		{
			"hello world",
			"{foo} {bar}",
			map[string]string{
				"foo": "hello",
				"bar": "world",
			},
		},
		{
			"foo: 42",
			"{x}: ${BOULEVARD_TEST_X}",
			map[string]string{
				"x":                "foo",
				"BOULEVARD_TEST_X": "wrong",
			},
		},
		{
			"foo \\{bar} \\$\\{baz}",
			"foo \\{bar} \\$\\{baz}",
			map[string]string{},
		},
	}

	for _, test := range tests {
		var s FormatString
		if err := s.Parse(test.s); err != nil {
			t.Errorf("cannot parse string %q: %v", test.s, err)
			continue
		}

		es := s.Expand(test.vars)
		if es != test.es {
			t.Errorf("%q expanded to %q but should have expanded to %q (%q)",
				test.s, es, test.es, s.parts)
		}
	}
}
