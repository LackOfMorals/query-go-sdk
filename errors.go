package query

import (
	"errors"

	"github.com/LackOfMorals/query-go-sdk/internal/api"
)

// Error represents an error response from the Aura API.
type Error = api.Error

// ErrorDetail represents individual error details.
type ErrorDetail = api.ErrorDetail

// IsNotFound reports whether err is an Aura API 404 Not Found error.
func IsNotFound(err error) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.IsNotFound()
}
