package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "test-secret-that-is-at-least-32-characters"

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("Correct1horse", 4)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "Correct1horse" || !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash does not look like bcrypt: %q", hash)
	}
	if !CheckPassword(hash, "Correct1horse") {
		t.Error("correct password rejected")
	}
	if CheckPassword(hash, "wrong1password") {
		t.Error("wrong password accepted")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	m := NewTokenManager(testSecret, "classora", time.Hour, 24*time.Hour)
	id := uuid.New()
	token, issued, err := m.Issue(id, "teacher", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, _ := claims.UserID(); got != id {
		t.Errorf("subject = %v, want %v", got, id)
	}
	if claims.Role != "teacher" || claims.Version != 3 || claims.ID != issued.ID {
		t.Errorf("unexpected claims: %+v", claims)
	}
	if ttl := time.Until(claims.ExpiresAt.Time); ttl > time.Hour || ttl < 59*time.Minute {
		t.Errorf("ttl = %v, want ~1h", ttl)
	}
}

func TestRememberMeUsesLongerTTL(t *testing.T) {
	m := NewTokenManager(testSecret, "classora", time.Hour, 24*time.Hour)
	_, claims, err := m.Issue(uuid.New(), "admin", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !claims.Remember || time.Until(claims.ExpiresAt.Time) < 23*time.Hour {
		t.Errorf("remember-me token should last ~24h, got %+v", claims)
	}
}

func TestExpiredToken(t *testing.T) {
	m := NewTokenManager(testSecret, "classora", time.Minute, time.Hour)
	m.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	token, _, err := m.Issue(uuid.New(), "admin", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	m.now = time.Now
	if _, err := m.Parse(token); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
}

func TestRejectsTamperedAndForeignTokens(t *testing.T) {
	m := NewTokenManager(testSecret, "classora", time.Hour, time.Hour)
	token, _, _ := m.Issue(uuid.New(), "student", 0, false)

	other := NewTokenManager("another-secret-that-is-at-least-32-chars", "classora", time.Hour, time.Hour)
	foreign, _, _ := other.Issue(uuid.New(), "admin", 0, false)

	wrongIssuer := NewTokenManager(testSecret, "someone-else", time.Hour, time.Hour)
	badIssuer, _, _ := wrongIssuer.Issue(uuid.New(), "admin", 0, false)

	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": uuid.NewString(), "jti": uuid.NewString(), "iss": "classora", "role": "admin",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]

	for name, tok := range map[string]string{
		"tampered":     tampered,
		"wrong secret": foreign,
		"wrong issuer": badIssuer,
		"alg none":     unsigned,
		"garbage":      "not-a-jwt",
		"empty":        "",
	} {
		if _, err := m.Parse(tok); !errors.Is(err, ErrTokenInvalid) {
			t.Errorf("%s: err = %v, want ErrTokenInvalid", name, err)
		}
	}
}
