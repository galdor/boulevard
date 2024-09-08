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
