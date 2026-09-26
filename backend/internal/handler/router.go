package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/aiclient"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/config"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/httpx"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/middleware"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/service"
)

const (
	defaultMaxBody = 64 << 10 // 64 KB
	adminMaxBody   = 1 << 20  // 1 MB, cukup untuk teks lowongan panjang
)

// NewRouter merakit repository -> service -> handler dan mendaftarkan semua route.
func NewRouter(cfg *config.Config, db *pgxpool.Pool, log *slog.Logger) *gin.Engine {
	taxonomyRepo := repository.NewTaxonomyRepository(db)
	sessionSvc := service.NewSessionService(repository.NewSessionRepository(db))
	taxonomySvc := service.NewTaxonomyService(taxonomyRepo)
	profileSvc := service.NewProfileService(repository.NewProfileRepository(db), taxonomyRepo)
	var processor service.JobProcessor
	if cfg.AIServiceURL != "" {
		processor = aiclient.New(cfg.AIServiceURL, cfg.InternalToken)
	}
	jobSvc := service.NewJobService(repository.NewJobRepository(db), processor, log)

	sessions := &SessionHandler{svc: sessionSvc}
	taxonomy := &TaxonomyHandler{svc: taxonomySvc}
	profile := &ProfileHandler{svc: profileSvc}
	adminJobs := &AdminJobHandler{svc: jobSvc}

	r := gin.New()
	// Backend diakses langsung (tanpa reverse proxy), jadi jangan percaya X-Forwarded-For.
	_ = r.SetTrustedProxies(nil)
	r.HandleMethodNotAllowed = true
	r.Use(middleware.RequestID(), middleware.RequestLog(log), gin.Recovery(), middleware.CORS(cfg.CORSOrigins))
	r.NoRoute(func(c *gin.Context) {
		httpx.AbortError(c, http.StatusNotFound, "not_found", "endpoint tidak ditemukan")
	})
	r.NoMethod(func(c *gin.Context) {
		httpx.AbortError(c, http.StatusMethodNotAllowed, "method_not_allowed", "method tidak didukung")
	})

	r.GET("/health", NewHealthHandler(db).Health)

	api := r.Group("/api", middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst).Middleware(middleware.ByIP))
	{
		api.POST("/sessions",
			middleware.PerMinute(cfg.SessionCreatePerMinute).Middleware(middleware.ByIP),
			sessions.Create)
		api.GET("/roles", taxonomy.ListRoles)
		api.GET("/skills", taxonomy.ListSkills)

		user := api.Group("", middleware.RequireSession(sessionSvc), middleware.MaxBody(defaultMaxBody))
		user.GET("/profile", profile.Get)
		user.PUT("/profile", profile.Put)

		admin := api.Group("/admin", middleware.RequireAdmin(cfg.AdminToken), middleware.MaxBody(adminMaxBody))
		admin.POST("/jobs", adminJobs.Create)
	}
	return r
}
