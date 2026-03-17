package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type tokenBucket struct {
	tokens   float64
	max      float64
	refill   float64 // tokens per second
	lastTime time.Time
	lastSeen time.Time
}

func (b *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.lastTime = now
	b.lastSeen = now

	b.tokens += elapsed * b.refill
	if b.tokens > b.max {
		b.tokens = b.max
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
	burst   float64
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		rps:     rps,
		burst:   float64(burst),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) get(ip string) *tokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{
			tokens:   rl.burst,
			max:      rl.burst,
			refill:   rl.rps,
			lastTime: time.Now(),
			lastSeen: time.Now(),
		}
		rl.buckets[ip] = b
	}
	return b
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			if time.Since(b.lastSeen) > 10*time.Minute {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		b := rl.get(ip)

		rl.mu.Lock()
		allowed := b.allow()
		rl.mu.Unlock()

		if !allowed {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
