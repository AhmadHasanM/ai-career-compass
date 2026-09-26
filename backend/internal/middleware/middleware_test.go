package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestRateLimiterBurstThenRefill(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(1, 2) // 1 req/detik, burst 2
	rl.now = func() time.Time { return now }

	if rl.reserve("a") != 0 || rl.reserve("a") != 0 {
		t.Fatal("dua request pertama seharusnya lolos (burst)")
	}
	if wait := rl.reserve("a"); wait <= 0 || wait > time.Second {
		t.Fatalf("request ketiga wait = %v, ingin (0, 1s]", wait)
	}
	if rl.reserve("b") != 0 {
		t.Fatal("key lain punya bucket sendiri")
	}
	now = now.Add(time.Second)
	if rl.reserve("a") != 0 {
		t.Fatal("setelah 1 detik token terisi lagi")
	}
}

func TestRateLimiterRejectionDoesNotConsumeToken(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(1, 1)
	rl.now = func() time.Time { return now }
	rl.reserve("a")
	for i := 0; i < 5; i++ {
		rl.reserve("a") // semua ditolak; tidak boleh menumpuk utang token
	}
	now = now.Add(time.Second)
	if rl.reserve("a") != 0 {
		t.Fatal("request yang ditolak tidak boleh memperpanjang antrean")
	}
}

func TestRateLimiterCleansIdleBuckets(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(1, 1)
	rl.now = func() time.Time { return now }
	rl.reserve("lama")
	now = now.Add(idleTTL + cleanupEvery + time.Second)
	rl.reserve("baru")
	if _, ok := rl.buckets["lama"]; ok {
		t.Error("bucket idle seharusnya dibersihkan")
	}
}

func TestRateLimitMiddlewareSetsRetryAfter(t *testing.T) {
	r := gin.New()
	r.GET("/", PerMinute(1).Middleware(ByIP), func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := make([]int, 2)
	var retry string
	for i := range codes {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		codes[i], retry = w.Code, w.Header().Get("Retry-After")
	}
	if codes[0] != http.StatusOK || codes[1] != http.StatusTooManyRequests || retry == "" {
		t.Errorf("codes = %v, Retry-After = %q", codes, retry)
	}
}

func TestRequireAdmin(t *testing.T) {
	cases := []struct {
		name, configured, sent string
		want                   int
	}{
		{"token benar", "rahasia", "rahasia", http.StatusOK},
		{"token salah", "rahasia", "tebakan", http.StatusUnauthorized},
		{"tanpa header", "rahasia", "", http.StatusUnauthorized},
		{"admin belum dikonfigurasi", "", "", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/", RequireAdmin(tc.configured), func(c *gin.Context) { c.Status(http.StatusOK) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.sent != "" {
				req.Header.Set(AdminHeader, tc.sent)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("status = %d, ingin %d", w.Code, tc.want)
			}
		})
	}
}

func TestRequestIDRejectsUnsafeValues(t *testing.T) {
	r := gin.New()
	r.GET("/", RequestID(), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "abc-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-Id"); got != "abc-123" {
		t.Errorf("request id aman harus dipakai ulang, dapat %q", got)
	}

	req.Header.Set("X-Request-Id", "bad id\ninjected")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-Id"); got == "" || got == "bad id\ninjected" {
		t.Errorf("request id tidak aman harus diganti, dapat %q", got)
	}
}
