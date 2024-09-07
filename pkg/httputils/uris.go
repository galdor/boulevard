package httputils

import (
	"net/url"
	"strings"

	"go.n16f.net/ejson"
)

func CheckHTTPURI(v *ejson.Validator, token interface{}, s string) bool {
	uri, err := url.Parse(s)
	if err != nil {
		v.AddError(token, "invalid_uri_format", "string must be a valid URI")
		return false
	}

	scheme := strings.ToLower(uri.Scheme)
	if scheme == "" {
		v.AddError(token, "missing_uri_scheme", "URI must have a scheme")
		return false
	}

	if scheme != "http" && scheme != "https" {
		v.AddError(token, "invalid_uri_scheme", "URI scheme must be either "+
			"\"http\" or \"https\"")
		return false
	}

	return true
}
