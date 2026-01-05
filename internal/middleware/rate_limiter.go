package middleware

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

const (
	// MaxLimitersPerInstance limits the number of IP limiters to prevent memory exhaustion attacks
	MaxLimitersPerInstance = 100000
	// CleanupInterval defines how often to clean up idle limiters
	CleanupInterval = time.Minute
	// IdleThreshold defines how long a limiter must be idle before cleanup (tokens fully replenished)
	IdleThreshold = 0.99
)

// RateLimiter implements a per-IP token bucket rate limiter with connection limits
type RateLimiter struct {
	enabled        bool
	limiters       map[string]*limiterEntry
	mu             sync.RWMutex
	r              rate.Limit
	b              int
	maxConns       int32
	connCount      int32
	cleanupStarted int32 // atomic flag to prevent multiple cleanup goroutines
}

// limiterEntry wraps a rate.Limiter with last access time for cleanup
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(enabled bool, requestsPerSecond int, burstSize int, maxConnections int) *RateLimiter {
	return &RateLimiter{
		enabled:  enabled,
		limiters: make(map[string]*limiterEntry),
		r:        rate.Limit(requestsPerSecond),
		b:        burstSize,
		maxConns: int32(maxConnections),
	}
}

// getLimiter returns or creates a rate limiter for the given IP
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	// Fast path: check if limiter exists with read lock
	rl.mu.RLock()
	entry, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if exists {
		// Update last access time (safe to do without lock for timestamp)
		entry.lastAccess = time.Now()
		return entry.limiter
	}

	// Slow path: create new limiter with write lock
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists := rl.limiters[ip]; exists {
		entry.lastAccess = time.Now()
		return entry.limiter
	}

	// Check map capacity to prevent memory exhaustion attacks
	if len(rl.limiters) >= MaxLimitersPerInstance {
		// Return a restrictive limiter for new IPs when at capacity
		return rate.NewLimiter(rate.Limit(1), 1)
	}

	limiter := rate.NewLimiter(rl.r, rl.b)
	rl.limiters[ip] = &limiterEntry{
		limiter:    limiter,
		lastAccess: time.Now(),
	}

	return limiter
}

// cleanupLimiters removes idle limiters to prevent memory leaks
func (rl *RateLimiter) cleanupLimiters() {
	// Use atomic CAS to ensure only one cleanup goroutine runs
	if !atomic.CompareAndSwapInt32(&rl.cleanupStarted, 0, 1) {
		return
	}

	ticker := time.NewTicker(CleanupInterval)
	go func() {
		for range ticker.C {
			rl.performCleanup()
		}
	}()
}

// performCleanup removes limiters that have been idle (tokens fully replenished)
func (rl *RateLimiter) performCleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	threshold := float64(rl.b) * IdleThreshold

	for ip, entry := range rl.limiters {
		// Check if limiter has been idle for at least 1 minute
		// and tokens are nearly full (without consuming any)
		if now.Sub(entry.lastAccess) > CleanupInterval {
			// Use Tokens() to check token count without consuming
			if entry.limiter.Tokens() >= threshold {
				delete(rl.limiters, ip)
			}
		}
	}
}

// extractClientIP safely extracts the client IP from the request
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by reverse proxies)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// The first IP is the original client
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// Check X-Real-IP header (set by some proxies like nginx)
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	// Fall back to RemoteAddr, removing port if present
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		// Check if it's IPv6 with brackets [::1]:port
		if bracketIdx := strings.LastIndex(ip, "]"); bracketIdx != -1 && bracketIdx < colonIdx {
			ip = ip[:colonIdx]
		} else if strings.Count(ip, ":") == 1 {
			// IPv4 with port
			ip = ip[:colonIdx]
		}
	}

	return ip
}

// Middleware returns an HTTP middleware that enforces rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	// If rate limiting is disabled, pass through directly
	if !rl.enabled {
		return next
	}

	// Start cleanup goroutine (idempotent due to atomic flag)
	rl.cleanupLimiters()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check connection limit using atomic operations
		for {
			current := atomic.LoadInt32(&rl.connCount)
			if current >= rl.maxConns {
				http.Error(w, "Too many connections", http.StatusTooManyRequests)
				return
			}
			if atomic.CompareAndSwapInt32(&rl.connCount, current, current+1) {
				break
			}
		}

		// Decrement connection count when request completes
		defer atomic.AddInt32(&rl.connCount, -1)

		// Extract client IP safely
		ip := extractClientIP(r)

		// Check rate limit
		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	currentConns := atomic.LoadInt32(&rl.connCount)

	rl.mu.RLock()
	activeLimiters := len(rl.limiters)
	rl.mu.RUnlock()

	return map[string]interface{}{
		"enabled":             rl.enabled,
		"active_limiters":     activeLimiters,
		"max_limiters":        MaxLimitersPerInstance,
		"current_connections": currentConns,
		"max_connections":     rl.maxConns,
		"requests_per_second": float64(rl.r),
		"burst_size":          rl.b,
	}
}
