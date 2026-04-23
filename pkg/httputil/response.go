package httputil

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/felipersas/payflow/pkg/errors"
	"github.com/felipersas/payflow/pkg/validation"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError maps an error to the appropriate HTTP response.
// DomainErrors carry their own status code; unknown errors become 500.
func WriteError(w http.ResponseWriter, err error) {
	var domErr *apperrors.DomainError
	if errors.As(err, &domErr) {
		WriteJSON(w, domErr.Code, map[string]string{"error": domErr.Message})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

// WriteValidationError writes a 422 with field-level details.
func WriteValidationError(w http.ResponseWriter, err error) {
	var valErr *validation.ValidationError
	if errors.As(err, &valErr) {
		WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  valErr.Message,
			"fields": valErr.Fields,
		})
		return
	}
	WriteJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
}

// DecodeAndValidate decodes JSON from the request body and validates the struct.
// Returns nil on success. Callers should check:
//   - validation error → WriteValidationError
//   - decode error → WriteJSON(400)
func DecodeAndValidate(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return decodeError{err}
	}
	return validation.Validate(dst)
}

type decodeError struct {
	Err error
}

func (e decodeError) Error() string { return e.Err.Error() }
func (e decodeError) Unwrap() error { return e.Err }

// IsDecodeError checks if the error is a JSON decode failure.
func IsDecodeError(err error) bool {
	var de decodeError
	return errors.As(err, &de)
}
