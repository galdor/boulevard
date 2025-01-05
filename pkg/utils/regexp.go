package utils

import (
	"fmt"
	"regexp"

	"go.n16f.net/bcl"
)

type Regexp struct {
	Regexp *regexp.Regexp
}

func (r *Regexp) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(string)
	case bcl.ValueTypeSymbol:
		s = string(v.Content.(bcl.Symbol))
	default:
		return v.ValueTypeError(bcl.ValueTypeString, bcl.ValueTypeSymbol)
	}

	re, err := regexp.Compile(s)
	if err != nil {
		return fmt.Errorf("invalid regexp: %w", err)
	}

	r.Regexp = re

	return nil
}

func (r *Regexp) MatchString(s string) bool {
	return r.Regexp.MatchString(s)
}
