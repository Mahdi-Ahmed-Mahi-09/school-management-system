// Package validation provides input normalisation and validation helpers.
package validation

import (
	"net/mail"
	"regexp"
	"strings"
	"time"
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

// PersonName validates a required person's name with a custom maximum length.
func PersonName(errs Errors, field, label, value string, max int) {
	if value == "" {
		errs.Add(field, label+" is required.")
		return
	}
	if utf8.RuneCountInString(value) > max {
		errs.Add(field, label+" is too long.")
		return
	}
	for _, r := range value {
		if unicode.IsControl(r) || strings.ContainsRune("<>{}[]\\", r) {
			errs.Add(field, label+" contains characters that are not allowed.")
			return
		}
	}
}

// Text validates free text: required-ness, maximum length and control
// characters (newlines are allowed only when multiline is true).
func Text(errs Errors, field, label, value string, max int, required, multiline bool) {
	if value == "" {
		if required {
			errs.Add(field, label+" is required.")
		}
		return
	}
	if utf8.RuneCountInString(value) > max {
		errs.Add(field, label+" is too long.")
		return
	}
	for _, r := range value {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r')) {
			errs.Add(field, label+" contains characters that are not allowed.")
			return
		}
	}
}

// Date parses a required YYYY-MM-DD calendar date.
func Date(errs Errors, field, label, value string) (time.Time, bool) {
	if value == "" {
		errs.Add(field, label+" is required.")
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		errs.Add(field, "Enter a valid date (YYYY-MM-DD).")
		return time.Time{}, false
	}
	return t, true
}

// NormalizeText trims surrounding whitespace and normalises line endings.
func NormalizeText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
