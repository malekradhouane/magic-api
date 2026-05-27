package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/utils/httpresp"
)

// rateLimitEntry tracks request timestamps for a single key.
type rateLimitEntry struct {
	timestamps []time.Time
}

// RateLimiter is a simple in-memory sliding-window rate limiter.
// For production, consider replacing with a Redis-backed implementation.
type RateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateLimitEntry
	max      int
	window   time.Duration
	cleanupT *time.Ticker
	done     chan struct{}
}

// NewRateLimiter creates a rate limiter that allows max requests per window.
func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		max:     max,
		window:  window,
		done:    make(chan struct{}),
	}
	rl.cleanupT = time.NewTicker(window)
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.cleanupT.C:
			rl.mu.Lock()
			now := time.Now()
			for k, e := range rl.entries {
				e.timestamps = filterRecent(e.timestamps, now, rl.window)
				if len(e.timestamps) == 0 {
					delete(rl.entries, k)
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			rl.cleanupT.Stop()
			return
		}
	}
}

func filterRecent(ts []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	out := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

// Allow checks whether the key is within rate limits. Returns true if allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	e, ok := rl.entries[key]
	if !ok {
		e = &rateLimitEntry{}
		rl.entries[key] = e
	}
	e.timestamps = filterRecent(e.timestamps, now, rl.window)
	if len(e.timestamps) >= rl.max {
		return false
	}
	e.timestamps = append(e.timestamps, now)
	return true
}

// Middleware returns a gin middleware that rate-limits by client IP.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if !rl.Allow(key) {
			httpresp.NewErrorMessage(c, http.StatusTooManyRequests, "too many requests, please try again later")
			c.Abort()
			return
		}
		c.Next()
	}
}
