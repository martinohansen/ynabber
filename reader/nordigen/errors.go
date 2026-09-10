package nordigen

import (
	"errors"
	"fmt"

	"github.com/frieser/nordigen-go-lib/v2"
)

// responseError keeps provider response bodies out of the error text without
// losing the original error identity or rate-limit metadata.
type responseError struct {
	message string
	cause   error
}

func (e *responseError) Error() string { return e.message }
func (e *responseError) Unwrap() error { return e.cause }

func apiResponseError(err error) error {
	var apiErr *nordigen.APIError
	if errors.As(err, &apiErr) {
		return &responseError{
			message: fmt.Sprintf("nordigen API returned status %d", apiErr.StatusCode),
			cause:   err,
		}
	}
	return err
}
