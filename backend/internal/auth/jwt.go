package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrTokenExpired means the token was valid but has expired.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenInvalid means the token is malformed, tampered with or otherwise unusable.
	ErrTokenInvalid = errors.New("token invalid")
)

// Claims are the JWT claims Classora issues.
//
// The role is included for convenience only; authorization always uses the
// role currently stored in the database so that changes apply immediately.
type Claims struct {
	Role     string `json:"role"`
	Version  int    `json:"ver"`
	Remember bool   `json:"rmb,omitempty"`
	jwt.RegisteredClaims
}

// UserID returns the subject as a UUID.
func (c *Claims) UserID() (uuid.UUID, error) { return uuid.Parse(c.Subject) }

// TokenID returns the JWT ID as a UUID.
func (c *Claims) TokenID() (uuid.UUID, error) { return uuid.Parse(c.ID) }

// TokenManager issues and verifies HMAC-signed JWTs.
type TokenManager struct {
	secret      []byte
	issuer      string
	ttl         time.Duration
	rememberTTL time.Duration
	now         func() time.Time
}

// NewTokenManager creates a TokenManager.
func NewTokenManager(secret, issuer string, ttl, rememberTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:      []byte(secret),
		issuer:      issuer,
		ttl:         ttl,
		rememberTTL: rememberTTL,
		now:         time.Now,
	}
}

// Issue signs a new token for the user.
func (m *TokenManager) Issue(userID uuid.UUID, role string, version int, remember bool) (string, *Claims, error) {
	now := m.now()
	ttl := m.ttl
	if remember {
		ttl = m.rememberTTL
	}
	claims := &Claims{
		Role:     role,
		Version:  version,
		Remember: remember,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   userID.String(),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", nil, err
	}
	return signed, claims, nil
}

// Parse verifies a token's signature, algorithm, issuer and lifetime.
func (m *TokenManager) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if !parsed.Valid {
		return nil, ErrTokenInvalid
	}
	if _, err := claims.UserID(); err != nil {
		return nil, ErrTokenInvalid
	}
	if _, err := claims.TokenID(); err != nil {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
