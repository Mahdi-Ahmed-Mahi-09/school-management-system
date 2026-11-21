// Package apperror defines typed errors that map cleanly to HTTP responses.
package apperror

import "net/http"

// Error is an error that is safe to show to API clients.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	// Err is the internal cause; it is logged but never sent to clients.
	Err error `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// WithCode returns a copy of the error with a more specific code.
func (e *Error) WithCode(code string) *Error {
	cp := *e
	cp.Code = code
	return &cp
}

func newErr(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// BadRequest is returned for malformed requests.
func BadRequest(message string) *Error {
	return newErr(http.StatusBadRequest, "BAD_REQUEST", message)
}

// Validation is returned when one or more fields are invalid.
func Validation(details map[string]string) *Error {
	e := newErr(http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Some fields need your attention.")
	e.Details = details
	return e
}

// Unauthorized is returned when authentication is missing or invalid.
func Unauthorized(code, message string) *Error {
	return newErr(http.StatusUnauthorized, code, message)
}

// Forbidden is returned when the caller lacks permission.
func Forbidden(message string) *Error {
	return newErr(http.StatusForbidden, "FORBIDDEN", message)
}

// NotFound is returned when a resource does not exist.
func NotFound(message string) *Error {
	return newErr(http.StatusNotFound, "NOT_FOUND", message)
}

// Conflict is returned when a request conflicts with existing state.
func Conflict(code, message string, details map[string]string) *Error {
	e := newErr(http.StatusConflict, code, message)
	e.Details = details
	return e
}

// TooManyRequests is returned when a client is rate limited.
func TooManyRequests(message string) *Error {
	return newErr(http.StatusTooManyRequests, "RATE_LIMITED", message)
}

// Internal wraps an unexpected error without leaking its details.
func Internal(err error) *Error {
	e := newErr(http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
	e.Err = err
	return e
}
