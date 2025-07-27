package httputils

import (
	"errors"
	"net/url"
)

func UnwrapUrlError(err error) error {
	var urlErr *url.Error

	if errors.As(err, &urlErr) {
		return urlErr.Err
	}

	return err
}
