// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every runtime setting the API needs.
type Config struct {
	Env         string
	Port        string
	DatabaseURL string

	JWTSecret      string
	JWTIssuer      string
	JWTTTL         time.Duration
	JWTRememberTTL time.Duration

	CookieName   string
	CookieSecure bool
	CookieDomain string

	CORSAllowedOrigins []string
	BcryptCost         int
	AutoMigrate        bool

	LoginRateLimit  int
	LoginRateWindow time.Duration
}

// IsProduction reports whether the app runs in production mode.
func (c *Config) IsProduction() bool { return c.Env == "production" }

// Load reads configuration from the environment (and an optional .env file).
func Load() (*Config, error) {
	// A missing .env file is fine: real deployments use the process environment.
	_ = godotenv.Load()

	env := getEnv("APP_ENV", "development")
	cfg := &Config{
		Env:                env,
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		JWTIssuer:          getEnv("JWT_ISSUER", "classora"),
		CookieName:         getEnv("SESSION_COOKIE_NAME", "classora_session"),
		CookieDomain:       os.Getenv("SESSION_COOKIE_DOMAIN"),
		CORSAllowedOrigins: splitList(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
	}

	var err error
	if cfg.JWTTTL, err = getDuration("JWT_TTL", 12*time.Hour); err != nil {
		return nil, err
	}
	if cfg.JWTRememberTTL, err = getDuration("JWT_REMEMBER_TTL", 7*24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.LoginRateWindow, err = getDuration("LOGIN_RATE_WINDOW", 15*time.Minute); err != nil {
		return nil, err
	}
	if cfg.CookieSecure, err = getBool("SESSION_COOKIE_SECURE", env == "production"); err != nil {
		return nil, err
	}
	if cfg.AutoMigrate, err = getBool("AUTO_MIGRATE", env != "production"); err != nil {
		return nil, err
	}
	if cfg.BcryptCost, err = getInt("BCRYPT_COST", 12); err != nil {
		return nil, err
	}
	if cfg.LoginRateLimit, err = getInt("LOGIN_RATE_LIMIT", 10); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	var problems []string
	if c.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		problems = append(problems, "JWT_SECRET is required and must be at least 32 characters")
	}
	if c.BcryptCost < 10 || c.BcryptCost > 15 {
		problems = append(problems, "BCRYPT_COST must be between 10 and 15")
	}
	if c.JWTTTL <= 0 || c.JWTRememberTTL <= 0 {
		problems = append(problems, "JWT_TTL and JWT_REMEMBER_TTL must be positive")
	}
	if c.LoginRateLimit <= 0 || c.LoginRateWindow <= 0 {
		problems = append(problems, "LOGIN_RATE_LIMIT and LOGIN_RATE_WINDOW must be positive")
	}
	if c.IsProduction() && !c.CookieSecure {
		problems = append(problems, "SESSION_COOKIE_SECURE must be true in production")
	}
	if len(problems) > 0 {
		return errors.New("invalid configuration: " + strings.Join(problems, "; "))
	}
	return nil
}

// LoadDatabaseURL is used by CLI tools (migrate, seed) that only need the database.
func LoadDatabaseURL() (string, error) {
	_ = godotenv.Load()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return "", errors.New("DATABASE_URL is required")
	}
	return url, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := getEnv(key, "")
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 12h or 30m", key)
	}
	return d, nil
}

func getBool(key string, fallback bool) (bool, error) {
	v := getEnv(key, "")
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}

func getInt(key string, fallback int) (int, error) {
	v := getEnv(key, "")
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return n, nil
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, strings.TrimRight(p, "/"))
		}
	}
	return out
}
