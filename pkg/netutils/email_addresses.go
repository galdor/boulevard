package netutils

import (
	"fmt"
	"net/mail"
)

func ValidateBCLEmailAddress(v any) error {
	s := v.(string)

	_, err := mail.ParseAddress(s)
	if err != nil {
		return fmt.Errorf("invalid email address: %w", err)
	}

	return nil
}
