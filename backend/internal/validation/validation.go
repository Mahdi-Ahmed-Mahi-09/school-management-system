// Package validation provides input normalisation and validation helpers.
package validation

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Errors maps a field name to a human-readable message.
type Errors map[string]string

// Add records a message for a field, keeping the first message per field.
func (e Errors) Add(field, message string) {
	if _, exists := e[field]; !exists {
		e[field] = message
	}
}

// Any reports whether at least one error was recorded.
func (e Errors) Any() bool { return len(e) > 0 }

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)
	phonePattern    = regexp.MustCompile(`^\+?[0-9][0-9 ()-]{5,18}[0-9]$`)
)

const (
	MaxNameLength     = 80
	MaxEmailLength    = 254
	MinPasswordLength = 8
	// MaxPasswordBytes is bcrypt's input limit.
	MaxPasswordBytes = 72
)

// NormalizeEmail trims and lowercases an email address.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// NormalizeUsername trims and lowercases a username.
func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// NormalizeName trims and collapses internal whitespace.
func NormalizeName(s string) string { return strings.Join(strings.Fields(s), " ") }

// Email validates an already-normalised email address.
func Email(errs Errors, field, value string) {
	switch {
	case value == "":
		errs.Add(field, "Email is required.")
	case len(value) > MaxEmailLength:
		errs.Add(field, "Email is too long.")
	default:
		addr, err := mail.ParseAddress(value)
		if err != nil || addr.Address != value || !strings.Contains(value[strings.LastIndex(value, "@"):], ".") {
			errs.Add(field, "Enter a valid email address.")
		}
	}
}

// Username validates an already-normalised username.
func Username(errs Errors, field, value string) {
	switch {
	case value == "":
		errs.Add(field, "Username is required.")
	case !usernamePattern.MatchString(value):
		errs.Add(field, "Use 3–32 lowercase letters, numbers, dots, dashes or underscores, starting with a letter or number.")
	}
}

// Name validates a person's first or last name.
func Name(errs Errors, field, label, value string) {
	switch {
	case value == "":
		errs.Add(field, label+" is required.")
	case utf8.RuneCountInString(value) > MaxNameLength:
		errs.Add(field, label+" must be at most 80 characters.")
	default:
		for _, r := range value {
			if unicode.IsControl(r) || strings.ContainsRune("<>{}[]\\", r) {
				errs.Add(field, label+" contains characters that are not allowed.")
				return
			}
		}
	}
}

// Phone validates an optional, already-trimmed phone number.
func Phone(errs Errors, field, value string) {
	if value != "" && !phonePattern.MatchString(value) {
		errs.Add(field, "Enter a valid phone number.")
	}
}

// Password validates password strength.
func Password(errs Errors, field, value string) {
	switch {
	case value == "":
		errs.Add(field, "Password is required.")
	case utf8.RuneCountInString(value) < MinPasswordLength:
		errs.Add(field, "Password must be at least 8 characters.")
	case len(value) > MaxPasswordBytes:
		errs.Add(field, "Password must be at most 72 bytes.")
	default:
		var letter, digit bool
		for _, r := range value {
			switch {
			case unicode.IsLetter(r):
				letter = true
			case unicode.IsDigit(r):
				digit = true
			}
		}
		if !letter || !digit {
			errs.Add(field, "Password must contain at least one letter and one number.")
		}
	}
}
