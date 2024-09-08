package httputils

import "strings"

func SplitTokenList(s string) []string {
	// See RFC 9110 5.6.1 Lists
	//
	// If we were implementing a proper parser, we would make sure that tokens
	// only contain valid characters (e.g. "foo bar" is not a proper token due
	// to the space character). But currently we do not care and do not want to
	// reject a request just because it contains an invalid token somewhere.
	//
	// Hence the name SplitTokenList and not ParseTokenList, and the absence of
	// a returned error.

	tokens := []string{}

	parts := strings.Split(s, ",")
	for _, part := range parts {
		if token := strings.Trim(part, " \t"); token != "" {
			tokens = append(tokens, token)
		}
	}

	return tokens
}
