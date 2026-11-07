package http

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter is an in-memory token bucket per key (usually the client IP). In-memory is
// deliberate for a single-instance deployment; a second app instance would need a
// shared store, which is the point at which this should move to Postgres or Redis.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   rate.Limit
	burst   int
	idleTTL time.Duration
	nowFunc func() time.Time // injectable for tests
}

type bucket struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// NewLimiter allows `burst` requests immediately, then refills at `every`.
func NewLimiter(every time.Duration, burst int) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		limit:   rate.Every(every),
		burst:   burst,
		idleTTL: 10 * time.Minute,
		nowFunc: time.Now,
	}
}

// WithIdleTTL keeps a quiet bucket for at least d before cleanup forgets it. A bucket
// that refills slowly must outlive its refill time, or forgetting it would hand an
// attacker a fresh, full bucket early.
func (l *Limiter) WithIdleTTL(d time.Duration) *Limiter {
	l.idleTTL = d
	return l
}

// HasToken reports whether key could pass right now, without consuming anything. Used
// by the per-account login throttle, which only spends tokens on failed attempts.
func (l *Limiter) HasToken(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		return true
	}
	return b.lim.TokensAt(l.nowFunc()) >= 1
}

// Allow consumes one token for key. An empty key (unresolvable IP) is refused rather
// than exempted — failing closed is the safer default for a limiter.
func (l *Limiter) Allow(key string) bool {
	if key == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{lim: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[key] = b
	}
	b.lastSeen = l.nowFunc()
	return b.lim.Allow()
}

// StartCleanup drops buckets that have gone quiet, so the map cannot grow without
// bound under a distributed flood. Stops with the context.
func (l *Limiter) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(l.idleTTL)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cutoff := l.nowFunc().Add(-l.idleTTL)
				l.mu.Lock()
				for key, b := range l.buckets {
					if b.lastSeen.Before(cutoff) {
						delete(l.buckets, key)
					}
				}
				l.mu.Unlock()
			}
		}
	}()
}

// Middleware refuses over-limit requests with 429 before any handler work happens.
func (s *Server) rateLimit(l *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(limiterKey(s.clientIP(r))) {
				s.log.Warn("rate limited",
					"request_id", RequestIDFrom(r.Context()),
					"path", r.URL.Path)
				w.Header().Set("Retry-After", "60")
				s.renderError(w, r, http.StatusTooManyRequests, "error.rate_limited")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limiterKey groups IPv6 clients by /64: one subscriber usually holds a whole /64, so
// keying on the full address would give a single attacker billions of fresh buckets.
// IPv4 addresses are used as they are.
func limiterKey(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip // "" stays "", which Allow refuses
	}
	if parsed.To4() != nil {
		return parsed.String()
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64"
}
