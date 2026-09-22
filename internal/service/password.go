package service

import (
	"strings"
	"unicode/utf8"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// Password policy: length over composition rules, per current NIST guidance. Long
// passphrases beat forced symbols, and argon2id makes offline cracking expensive.
const (
	MinPasswordLength = 12
	MaxPasswordLength = 1024 // bounded so a huge input cannot be used to burn CPU
)

// ValidatePassword returns a translatable error wrapping domain.ErrValidation.
// It never echoes the password.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return FieldMessage(domain.ErrValidation, "validation.password_too_short", "field.password", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return tooLong("field.password", MaxPasswordLength)
	}
	if strings.TrimSpace(password) == "" {
		return required("field.password")
	}
	return nil
}

// ValidateEmail checks what the DB CHECK also enforces, so the user gets a message
// instead of a constraint violation.
func ValidateEmail(email string) error {
	if email == "" {
		return required("field.email")
	}
	if len(email) > 254 {
		return tooLong("field.email", 254)
	}
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 || strings.ContainsAny(email, " \t\r\n") || strings.Count(email, "@") != 1 {
		return badFormat("field.email")
	}
	if email != NormalizeEmail(email) {
		return badFormat("field.email")
	}
	return nil
}
