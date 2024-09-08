package httputils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitTokenList(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		s      string
		tokens []string
	}{
		{"",
			[]string{}},
		{"foo",
			[]string{"foo"}},
		{"foo,bar",
			[]string{"foo", "bar"}},
		{"foo ,	bar		 ,baz",
			[]string{"foo", "bar", "baz"}},
		{"	 foo,bar  ",
			[]string{"foo", "bar"}},
		{",,foo	,	 ,,bar,",
			[]string{"foo", "bar"}},
		{"foo ,bar	baz",
			[]string{"foo", "bar	baz"}},
	}

	for _, test := range tests {
		label := fmt.Sprintf("%q", test.s)

		tokens := SplitTokenList(test.s)
		assert.Equal(test.tokens, tokens, label)
	}
}

func TestAppendToTokenList(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		s      string
		tokens []string
		result string
	}{
		{"",
			[]string{},
			""},
		{"foo",
			[]string{},
			"foo"},
		{"foo",
			[]string{"bar"},
			"foo, bar"},
		{"foo, bar",
			[]string{"a", "b", "c"},
			"foo, bar, a, b, c"},
		{"foo	,,",
			[]string{"bar"},
			"foo, bar"},
	}

	for _, test := range tests {
		label := fmt.Sprintf("%q %v", test.s, test.tokens)

		result := AppendToTokenList(test.s, test.tokens...)
		assert.Equal(test.result, result, label)
	}
}
