package profile

import "strings"

// FieldError points at one invalid field using its JSON path, so the setup
// page can show the message next to the input.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError collects every problem found, not just the first.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fe.Field+": "+fe.Message)
	}
	return "invalid profile: " + strings.Join(parts, "; ")
}

func (e *ValidationError) add(field, message string) {
	e.Errors = append(e.Errors, FieldError{Field: field, Message: message})
}
