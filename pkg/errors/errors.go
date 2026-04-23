package errors

import "fmt"

// DomainError represents a domain-level error with an HTTP status code.
// Handlers use errors.As to extract the status, eliminating status-code guessing.
type DomainError struct {
	Code    int
	Message string
}

func (e *DomainError) Error() string { return e.Message }

func NotFound(format string, args ...any) *DomainError {
	return &DomainError{404, fmt.Sprintf(format, args...)}
}

func Conflict(format string, args ...any) *DomainError {
	return &DomainError{409, fmt.Sprintf(format, args...)}
}

func BusinessRule(format string, args ...any) *DomainError {
	return &DomainError{422, fmt.Sprintf(format, args...)}
}

func Unauthorized(msg string) *DomainError {
	return &DomainError{401, msg}
}

func Forbidden(msg string) *DomainError {
	return &DomainError{403, msg}
}
