package service

import (
	"errors"
	"fmt"
)

// MessageError is an error the UI can show. It carries an i18n message id rather than
// English text, so the same failure renders in en, lv or ru without the service layer
// knowing anything about locales.
//
//	Key      — message id, e.g. "validation.required" or "error.bus_not_active"
//	FieldKey — optional message id for the field name, substituted as the first argument
//	Args     — further substitution arguments (numbers, names already safe to print)
//	Sentinel — the domain error to branch on (ErrValidation, ErrConflict, ...)
//
// Error() renders a plain-English fallback for logs; the browser never sees it.
type MessageError struct {
	Key      string
	FieldKey string
	Args     []any
	Sentinel error
}

func (e *MessageError) Error() string {
	if e.FieldKey != "" {
		return fmt.Sprintf("%s (%s): %v", e.Key, e.FieldKey, e.Sentinel)
	}
	return fmt.Sprintf("%s: %v", e.Key, e.Sentinel)
}

func (e *MessageError) Unwrap() error { return e.Sentinel }

// Message builds a translatable error for a given domain sentinel.
func Message(sentinel error, key string, args ...any) *MessageError {
	return &MessageError{Key: key, Args: args, Sentinel: sentinel}
}

// FieldMessage builds a translatable validation error about a specific field.
func FieldMessage(sentinel error, key, fieldKey string, args ...any) *MessageError {
	return &MessageError{Key: key, FieldKey: fieldKey, Args: args, Sentinel: sentinel}
}

// AsMessage extracts a MessageError from a wrapped error chain, if there is one.
func AsMessage(err error) (*MessageError, bool) {
	var msg *MessageError
	if errors.As(err, &msg) {
		return msg, true
	}
	return nil, false
}
