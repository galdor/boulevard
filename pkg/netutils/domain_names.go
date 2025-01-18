package netutils

import (
	"fmt"
	"strings"
)

func ValidateBCLDomainName(v any) error {
	// RFC 952 DOD INTERNET HOST TABLE SPECIFICATION
	//
	// <domainname> ::= <hname>
	// <hname> ::= <name>*["."<name>]
	// <name>  ::= <let>[*[<let-or-digit-or-hyphen>]<let-or-digit>]

	// RFC 1034 3.5. Preferred name syntax
	//
	// "Labels must be 63 characters or less"

	// RFC 1123 2. GENERAL ISSUES
	//
	// "the restriction on the first character is relaxed to allow either a
	// letter or a digit. Host software MUST support this more liberal syntax."

	const maxLabelLength = 63

	isLetter := func(c rune) bool {
		return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
	}

	isDigit := func(c rune) bool {
		return c >= '0' && c <= '9'
	}

	s := v.(string)

	if s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	for _, label := range strings.Split(s, ".") {
		if len(label) == 0 {
			return fmt.Errorf("invalid empty domain name label")
		}

		cs := []rune(label)

		for _, c := range cs {
			if c > 0x7f {
				return fmt.Errorf("invalid domain name label %q: "+
					"invalid character %q", label, c)
			}
		}

		if len(label) > maxLabelLength {
			return fmt.Errorf("invalid domain name label %q: "+
				"labels must contain less than %d characters",
				label, maxLabelLength+1)
		}

		if c := cs[0]; !(isLetter(c) || isDigit(c)) {
			return fmt.Errorf("invalid domain name label %q: "+
				"labels must start with a letter or digit", label)
		}

		if c := cs[len(cs)-1]; !(isLetter(c) || isDigit(c)) {
			return fmt.Errorf("invalid domain name label %q: "+
				"labels must end with a letter or digit", label)
		}

		for i := 1; i < len(cs)-1; i++ {
			if c := cs[i]; !(isLetter(c) || isDigit(c) || c == '-') {
				return fmt.Errorf("invalid domain name label %q: "+
					"labels characters must be letters, digits or '-' characters",
					label)
			}
		}
	}

	return nil
}
