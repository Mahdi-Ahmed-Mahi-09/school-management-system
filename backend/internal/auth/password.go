// Package auth contains password hashing and JWT token primitives.
package auth

import (
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password with bcrypt at the given cost.
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

var (
	dummyOnce sync.Once
	dummyHash []byte
)

// EqualizeTiming performs a bcrypt comparison against a throwaway hash so
// that "unknown user" and "wrong password" take a similar amount of time,
// which prevents account enumeration through response timing.
func EqualizeTiming(password string, cost int) {
	dummyOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("classora-timing-equalizer"), cost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
