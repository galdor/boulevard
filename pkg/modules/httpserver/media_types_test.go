package httpserver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMediaTypeParsing(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		s  string
		t  *MediaType
		ns string
	}{
		{"text/plain",
			&MediaType{Type: "text", Subtype: "plain"},
			"text/plain"},
		{" text	/ 	 plain	",
			&MediaType{Type: "text", Subtype: "plain"},
			"text/plain"},
		{"text/plain; charset=utf-8",
			&MediaType{Type: "text", Subtype: "plain",
				Parameters: []MediaTypeParameter{
					{"charset", "utf-8"},
				}},
			"text/plain;charset=utf-8"},
		{"text/plain	;  charset =	utf-8 ;foo=bar ",
			&MediaType{Type: "text", Subtype: "plain",
				Parameters: []MediaTypeParameter{
					{"charset", "utf-8"},
					{"foo", "bar"},
				}},
			"text/plain;charset=utf-8;foo=bar"},

		{"foo", nil, ""},
		{"foo/", nil, ""},
		{"/bar", nil, ""},
		{"foo/bar;", nil, ""},
		{"foo/bar; a", nil, ""},
		{"foo/bar; a=", nil, ""},
		{"foo/bar; =b", nil, ""},
		{"foo/bar;; a=b", nil, ""},
	}

	for _, test := range tests {
		label := fmt.Sprintf("%q", test.s)

		var mt MediaType
		err := mt.Parse(test.s)

		if test.t == nil {
			assert.Error(err, label)
		} else {
			if assert.NoError(err, label) {
				assert.Equal(test.t, &mt, label)
				assert.Equal(test.ns, mt.String(), label)
			}
		}
	}
}

func TestMediaRangeParsing(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		s  string
		r  *MediaRange
		ns string
	}{
		{"text/plain",
			&MediaRange{Type: "text", Subtype: "plain",
				Quality: 1.0},
			"text/plain"},
		{"text/*",
			&MediaRange{Type: "text", Subtype: "",
				Quality: 1.0},
			"text/*"},
		{"*/*",
			&MediaRange{Type: "", Subtype: "",
				Quality: 1.0},
			"*/*"},
		{"text/plain; q=0.25	;charset=utf-8",
			&MediaRange{Type: "text", Subtype: "plain",
				Quality: 0.25,
				Parameters: []MediaTypeParameter{
					{"charset", "utf-8"},
				}},
			"text/plain;charset=utf-8;q=0.25"},

		{"foo", nil, ""},
		{"foo/", nil, ""},
		{"/bar", nil, ""},
		{"*/bar;", nil, ""},
		{"foo/bar;", nil, ""},
		{"foo/bar; a", nil, ""},
		{"foo/bar; a=", nil, ""},
		{"foo/bar; =b", nil, ""},
		{"foo/bar;; a=b", nil, ""},
	}

	for _, test := range tests {
		label := fmt.Sprintf("%q", test.s)

		var r MediaRange
		err := r.Parse(test.s)

		if test.r == nil {
			assert.Error(err, label)
		} else {
			if assert.NoError(err, label) {
				assert.Equal(test.r, &r, label)
				assert.Equal(test.ns, r.String(), label)
			}
		}
	}
}
