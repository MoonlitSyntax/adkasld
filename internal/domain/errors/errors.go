package errors

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalid = errors.New("invalid")

type FieldError struct {
	Field   string
	Message string
}

func (e FieldError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationError struct {
	Items []FieldError
}

func (e ValidationError) Error() string {
	if len(e.Items) == 0 {
		return "validation failed"
	}

	var b strings.Builder
	b.WriteString("validation failed:\n")
	for _, item := range e.Items {
		b.WriteString(" - ")
		b.WriteString(item.Error())
		b.WriteString("\n")
	}
	return b.String()
}

func (e *ValidationError) Add(field, msg string) {
	e.Items = append(e.Items, FieldError{
		Field:   field,
		Message: msg,
	})
}

func (e ValidationError) Is(target error) bool {
	return target == ErrInvalid
}

func (e ValidationError) HasAny() bool {
	return len(e.Items) > 0
}
