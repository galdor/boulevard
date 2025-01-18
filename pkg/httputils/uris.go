package httputils

import (
	"fmt"
	"net/url"
	"strings"
)

func ValidateBCLHTTPURI(v any) error {
	s := v.(string)

	uri, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	scheme := strings.ToLower(uri.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("invalid non-HTTP URI")
	}

	return nil
}
