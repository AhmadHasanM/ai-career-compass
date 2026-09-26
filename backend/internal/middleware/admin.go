package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/httpx"
)

const AdminHeader = "X-Admin-Token"

// RequireAdmin mencocokkan X-Admin-Token secara constant-time.
// Jika ADMIN_TOKEN tidak di-set, semua endpoint admin ditutup.
func RequireAdmin(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			httpx.AbortError(c, http.StatusServiceUnavailable, "admin_disabled", "ADMIN_TOKEN belum dikonfigurasi")
			return
		}
		got := c.GetHeader(AdminHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			httpx.AbortError(c, http.StatusUnauthorized, "admin_unauthorized", "X-Admin-Token tidak valid")
			return
		}
		c.Next()
	}
}
