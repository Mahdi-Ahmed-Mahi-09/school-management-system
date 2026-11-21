package validation

import (
	"strings"
	"testing"
)

func TestEmail(t *testing.T) {
	cases := map[string]bool{
		"jane@school.edu":                  true,
		"j.doe+tag@sub.school.bd":          true,
		"":                                 false,
		"no-at-sign":                       false,
		"jane@localhost":                   false,
		"Jane <jane@school.edu>":           false,
		"a@b.c" + strings.Repeat("x", 260): false,
	}
	for in, valid := range cases {
		errs := Errors{}
		Email(errs, "email", in)
		if errs.Any() == valid {
			t.Errorf("Email(%q) valid=%v, errors=%v", in, valid, errs)
		}
	}
}

func TestUsername(t *testing.T) {
	cases := map[string]bool{
		"jane":                  true,
		"jane.doe_1":            true,
		"ab":                    false,
		"-jane":                 false,
		"jane doe":              false,
		"jane@x":                false,
		strings.Repeat("a", 33): false,
	}
	for in, valid := range cases {
		errs := Errors{}
		Username(errs, "username", in)
		if errs.Any() == valid {
			t.Errorf("Username(%q) valid=%v, errors=%v", in, valid, errs)
		}
	}
}

func TestPassword(t *testing.T) {
	cases := map[string]bool{
		"Secret123":              true,
		"short1":                 false,
		"onlyletters":            false,
		"12345678":               false,
		strings.Repeat("a1", 40): false,
	}
	for in, valid := range cases {
		errs := Errors{}
		Password(errs, "password", in)
		if errs.Any() == valid {
			t.Errorf("Password(%q) valid=%v, errors=%v", in, valid, errs)
		}
	}
}

func TestNameAndPhone(t *testing.T) {
	errs := Errors{}
	Name(errs, "first_name", "First name", "<script>")
	Phone(errs, "phone", "call me")
	if errs["first_name"] == "" || errs["phone"] == "" {
		t.Errorf("expected errors, got %v", errs)
	}

	ok := Errors{}
	Name(ok, "first_name", "First name", "Nusrat Jahan")
	Phone(ok, "phone", "+880 1712-345678")
	if ok.Any() {
		t.Errorf("unexpected errors: %v", ok)
	}
}

func TestNormalizers(t *testing.T) {
	if got := NormalizeEmail("  Jane@School.EDU "); got != "jane@school.edu" {
		t.Errorf("NormalizeEmail = %q", got)
	}
	if got := NormalizeName("  Mary   Ann  "); got != "Mary Ann" {
		t.Errorf("NormalizeName = %q", got)
	}
}
