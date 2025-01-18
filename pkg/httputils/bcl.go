package httputils

import (
	"fmt"
	"slices"
)

var methods = []string{
	// RFC 9110
	"GET",
	"HEAD",
	"POST",
	"PUT",
	"DELETE",
	"CONNECT",
	"OPTIONS",
	"TRACE",

	// RFC 5789
	"PATCH",
}

func ValidateBCLStatus(v any) error {
	status := v.(int)

	if status < 200 || status > 599 {
		return fmt.Errorf("invalid HTTP status")
	}

	return nil
}

func ValidateBCLMethod(v any) error {
	method := v.(string)

	if !slices.Contains(methods, method) {
		return fmt.Errorf("invalid HTTP method")
	}

	return nil
}
