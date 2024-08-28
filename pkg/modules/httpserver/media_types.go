package httpserver

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var (
	MediaTypeText = &MediaType{Type: "text", Subtype: "plain"}
	MediaTypeHTML = &MediaType{Type: "text", Subtype: "html"}
	MediaTypeJSON = &MediaType{Type: "application", Subtype: "json"}
)

type MediaType struct {
	Type       string
	Subtype    string
	Parameters []MediaTypeParameter
}

func (t *MediaType) String() string {
	var buf bytes.Buffer

	buf.WriteString(t.Type)
	buf.WriteByte('/')
	buf.WriteString(t.Subtype)

	for _, p := range t.Parameters {
		buf.WriteByte(';')
		buf.WriteString(p.String())
	}

	return buf.String()
}

func (t *MediaType) Parse(s string) error {
	slash := strings.IndexByte(s, '/')
	if slash == -1 {
		return fmt.Errorf("missing '/' separator")
	}

	mtype := strings.Trim(s[:slash], " \t")
	if mtype == "" {
		return fmt.Errorf("invalid empty type")
	}

	t.Type = mtype
	s = s[slash+1:]

	semicolon := strings.IndexByte(s, ';')
	end := semicolon
	if end == -1 {
		end = len(s)
	}

	subtype := strings.Trim(s[:end], " \t")
	if subtype == "" {
		return fmt.Errorf("invalid empty subtype")
	}

	t.Subtype = subtype

	if semicolon >= 0 {
		parts := strings.Split(s[semicolon+1:], ";")

		params := make([]MediaTypeParameter, len(parts))
		for i, part := range parts {
			if err := params[i].Parse(part); err != nil {
				return fmt.Errorf("invalid parameter: %w", err)
			}
		}

		t.Parameters = params
	}

	return nil
}

type MediaRange struct {
	Type       string // optional
	Subtype    string // optional
	Quality    float64
	Parameters []MediaTypeParameter
}

func (r *MediaRange) String() string {
	var buf bytes.Buffer

	if r.Type == "" {
		buf.WriteByte('*')
	} else {
		buf.WriteString(r.Type)
	}

	buf.WriteByte('/')

	if r.Subtype == "" {
		buf.WriteByte('*')
	} else {
		buf.WriteString(r.Subtype)
	}

	for _, p := range r.Parameters {
		buf.WriteByte(';')
		buf.WriteString(p.String())
	}

	// RFC 9110 12.5.1. "Each media-range might be followed by optional
	// applicable media type parameters (e.g., charset), followed by an optional
	// "q" parameter"
	//
	// So we add it after other parameters.
	if r.Quality != 1.0 {
		buf.WriteString(";q=")

		q := strconv.FormatFloat(float64(r.Quality), 'g', -1, 64)
		buf.WriteString(q)
	}

	return buf.String()
}

func (r *MediaRange) Parse(s string) error {
	var t MediaType
	if err := t.Parse(s); err != nil {
		return err
	}

	if t.Type == "*" && t.Subtype != "*" {
		return fmt.Errorf("invalid wildcard type with non-wildcard subtype")
	}

	if t.Type != "*" {
		r.Type = t.Type
	}

	if t.Subtype != "*" {
		r.Subtype = t.Subtype
	}

	r.Quality = 1.0

	for _, p := range t.Parameters {
		if strings.ToLower(p.Name) == "q" {
			f, err := strconv.ParseFloat(p.Value, 64)
			if err != nil {
				return fmt.Errorf("invalid \"q\" parameter")
			}

			r.Quality = f
		} else {
			param := MediaTypeParameter{Name: p.Name, Value: p.Value}
			r.Parameters = append(r.Parameters, param)
		}
	}

	return nil
}

func (r *MediaRange) Matches(t *MediaType) bool {
	if r.Type != "" && r.Type != t.Type {
		return false
	}

	if r.Subtype != "" && r.Subtype != t.Subtype {
		return false
	}

	return true
}

type MediaTypeParameter struct {
	Name  string
	Value string
}

func (p *MediaTypeParameter) String() string {
	return p.Name + "=" + p.Value
}

func (p *MediaTypeParameter) Parse(s string) error {
	equal := strings.IndexByte(s, '=')
	if equal == -1 {
		return fmt.Errorf("missing '=' separator")
	}

	name := strings.Trim(s[:equal], " \t")
	if name == "" {
		return fmt.Errorf("invalid empty name")
	}

	p.Name = name

	value := strings.Trim(s[equal+1:], " \t")
	if value == "" {
		return fmt.Errorf("invalid empty value")
	}

	p.Value = value

	return nil
}

func (r *MediaRange) PreferableTo(r2 *MediaRange) bool {
	if r.Quality > r2.Quality {
		return true
	}

	if r.Quality < r2.Quality {
		return false
	}

	// More specific media ranges are preferable to less specific ones (cf. RFC
	// 9110 12.5.1.).
	return r.specificityScore() > r2.specificityScore()
}

func (r *MediaRange) specificityScore() int {
	score := 0

	if r.Type != "" {
		score++
	}

	if r.Subtype != "" {
		score++
	}

	if len(r.Parameters) > 0 {
		score++
	}

	return score
}

func NegotiateMediaType(ranges []*MediaRange, supportedTypes []*MediaType) *MediaType {
	ranges = slices.Clone(ranges)
	sort.Slice(ranges, func(i, j int) bool {
		r1, r2 := ranges[i], ranges[j]
		return r1.PreferableTo(r2)
	})

	for _, r := range ranges {
		for _, t := range supportedTypes {
			if r.Matches(t) {
				return t
			}
		}
	}

	return supportedTypes[0]
}
