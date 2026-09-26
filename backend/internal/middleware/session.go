package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/httpx"
)

const (
	SessionHeader = "X-Session-Id"
	sessionIDKey  = "session_id"
)

type SessionToucher interface {
	Touch(ctx context.Context, id uuid.UUID) (bool, error)
}

// RequireSession memvalidasi header X-Session-Id terhadap tabel user_sessions.
func RequireSession(sessions SessionToucher) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader(SessionHeader)
		if raw == "" {
			httpx.AbortError(c, http.StatusUnauthorized, "session_required",
				"header X-Session-Id wajib; buat sesi lewat POST /api/sessions")
			return
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.AbortError(c, http.StatusUnauthorized, "session_invalid", "X-Session-Id bukan UUID yang valid")
			return
		}
		ok, err := sessions.Touch(c.Request.Context(), id)
		if err != nil {
			_ = c.Error(err)
			httpx.AbortError(c, http.StatusInternalServerError, "internal_error", "gagal memeriksa sesi")
			return
		}
		if !ok {
			httpx.AbortError(c, http.StatusUnauthorized, "session_invalid", "sesi tidak ditemukan; buat sesi baru")
			return
		}
		c.Set(sessionIDKey, id)
		c.Next()
	}
}

// SessionID mengambil id sesi yang sudah divalidasi RequireSession.
func SessionID(c *gin.Context) uuid.UUID {
	id, _ := c.Get(sessionIDKey)
	sid, _ := id.(uuid.UUID)
	return sid
}
