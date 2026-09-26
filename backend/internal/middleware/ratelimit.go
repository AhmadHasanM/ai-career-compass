package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/httpx"
)

// RateLimiter adalah token bucket per key (IP atau sesi), disimpan in-memory.
// Cukup untuk satu instance backend; jika di-scale horizontal perlu store bersama (Redis).
type RateLimiter struct {
	limit rate.Limit
	burst int
	now   func() time.Time

	mu          sync.Mutex
	buckets     map[string]*bucket
	lastCleanup time.Time
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

const (
	cleanupEvery = time.Minute
	idleTTL      = 10 * time.Minute
)

func NewRateLimiter(perSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		limit:   rate.Limit(perSecond),
		burst:   burst,
		now:     time.Now,
		buckets: map[string]*bucket{},
	}
}

// PerMinute membuat limiter n request per menit dengan burst n.
func PerMinute(n int) *RateLimiter {
	return NewRateLimiter(float64(n)/60, n)
}

// reserve mengembalikan 0 jika request boleh lewat, atau durasi tunggu jika ditolak.
func (rl *RateLimiter) reserve(key string) time.Duration {
	now := rl.now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if now.Sub(rl.lastCleanup) > cleanupEvery {
		for k, b := range rl.buckets {
			if now.Sub(b.lastSeen) > idleTTL {
				delete(rl.buckets, k)
			}
		}
		rl.lastCleanup = now
	}

	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.buckets[key] = b
	}
	b.lastSeen = now

	r := b.limiter.ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now)
		return delay
	}
	return 0
}

type KeyFunc func(*gin.Context) string

func ByIP(c *gin.Context) string { return "ip:" + c.ClientIP() }

// BySession memakai sesi yang sudah divalidasi RequireSession (harus dipasang sesudahnya).
func BySession(c *gin.Context) string { return "session:" + SessionID(c).String() }

// Middleware menolak request dengan 429 + Retry-After jika bucket key habis.
func (rl *RateLimiter) Middleware(key KeyFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if wait := rl.reserve(key(c)); wait > 0 {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			httpx.AbortError(c, http.StatusTooManyRequests, "rate_limited", "terlalu banyak request, coba lagi nanti")
			return
		}
		c.Next()
	}
}

// MaxBody membatasi ukuran body request (byte).
func MaxBody(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}
