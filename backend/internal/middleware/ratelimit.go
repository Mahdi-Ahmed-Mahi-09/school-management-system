package middleware

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
)

// RateLimiter is a fixed-window, in-memory limiter keyed by client IP.
// It is intended for a single API instance; a shared store would be needed
// when running several replicas.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter creates a limiter allowing limit requests per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, buckets: map[string]*bucket{}, now: time.Now}
}

// Allow records a hit for key and reports whether it is within the limit,
// along with the time until the window resets.
func (l *RateLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.buckets) > 10_000 {
		for k, b := range l.buckets {
			if now.After(b.resetAt) {
				delete(l.buckets, k)
			}
		}
	}
	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		b = &bucket{resetAt: now.Add(l.window)}
		l.buckets[key] = b
	}
	b.count++
	return b.count <= l.limit, b.resetAt.Sub(now)
}

// Middleware limits requests per client IP.
func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, retry := l.Allow(c.ClientIP())
		if !ok {
			seconds := int(retry.Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(seconds))
			response.Error(c, apperror.TooManyRequests(
				fmt.Sprintf("Too many sign-in attempts. Please try again in %d minute(s).", (seconds+59)/60)))
			return
		}
		c.Next()
	}
}
