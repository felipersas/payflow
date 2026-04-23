package validation

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is a package-level singleton. It's thread-safe and cached.
var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

// Validate validates a struct and returns a formatted error.
// Returns nil if the struct passes all validation rules.
func Validate(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}
	return formatError(err)
}

// ValidationError contains field-level validation details.
type ValidationError struct {
	Message string
	Fields  map[string]string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// formatError converts validator.ValidationErrors into a ValidationError
// with human-readable field messages in English.
func formatError(err error) *ValidationError {
	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return &ValidationError{
			Message: err.Error(),
			Fields:  map[string]string{"_": err.Error()},
		}
	}

	fields := make(map[string]string, len(validationErrs))
	for _, e := range validationErrs {
		field := toSnakeCase(e.Field())
		fields[field] = fieldMessage(e)
	}

	return &ValidationError{
		Message: fmt.Sprintf("validation failed: %s", joinFields(fields)),
		Fields:  fields,
	}
}

func fieldMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", e.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", e.Field(), e.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", e.Field(), e.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", e.Field(), e.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", e.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", e.Field(), e.Param())
	default:
		return fmt.Sprintf("%s failed %s validation", e.Field(), e.Tag())
	}
}

func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func joinFields(fields map[string]string) string {
	parts := make([]string, 0, len(fields))
	for _, v := range fields {
		parts = append(parts, v)
	}
	return strings.Join(parts, "; ")
}
