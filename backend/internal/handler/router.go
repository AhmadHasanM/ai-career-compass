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
	insightsRepo := repository.NewInsightsRepository(db)
	profileRepo := repository.NewProfileRepository(db)
	resourceRepo := repository.NewResourceRepository(db)

	sessionSvc := service.NewSessionService(repository.NewSessionRepository(db))
	taxonomySvc := service.NewTaxonomyService(taxonomyRepo)
	profileSvc := service.NewProfileService(profileRepo, taxonomyRepo)

	// Tanpa AI_SERVICE_URL (misal saat test) backend tetap jalan: job tidak diproses,
	// roadmap memakai penjelasan fallback.
	var (
		processor service.JobProcessor
		embedder  service.ResourceEmbedder
		explainer service.RoadmapExplainer
	)
	if cfg.AIServiceURL != "" {
		ai := aiclient.New(cfg.AIServiceURL, cfg.InternalToken)
		processor, embedder, explainer = ai, ai, ai
	}
	jobSvc := service.NewJobService(repository.NewJobRepository(db), processor, log)
	insightsSvc := service.NewInsightsService(insightsRepo)
	gapSvc := service.NewGapService(insightsRepo, profileRepo, service.DefaultGapConfig)
	roadmapSvc := service.NewRoadmapService(gapSvc, repository.NewRoadmapRepository(db), resourceRepo, explainer, log)
	resourceSvc := service.NewResourceService(resourceRepo, taxonomyRepo, embedder, log)

	sessions := &SessionHandler{svc: sessionSvc}
	taxonomy := &TaxonomyHandler{svc: taxonomySvc}
	profile := &ProfileHandler{svc: profileSvc}
	adminJobs := &AdminJobHandler{svc: jobSvc}
	insights := &InsightsHandler{svc: insightsSvc}
	gap := &GapHandler{svc: gapSvc}
	roadmap := &RoadmapHandler{svc: roadmapSvc}
	resources := &ResourceHandler{svc: resourceSvc}

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

	// Rate limit umum per IP berlaku untuk endpoint publik dan pengguna. Endpoint admin sudah
	// dilindungi token dan dipakai untuk ingestion massal, jadi tidak ikut dibatasi.
	api := r.Group("/api")
	public := api.Group("", middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst).Middleware(middleware.ByIP))
	{
		public.POST("/sessions",
			middleware.PerMinute(cfg.SessionCreatePerMinute).Middleware(middleware.ByIP),
			sessions.Create)
		public.GET("/roles", taxonomy.ListRoles)
		public.GET("/skills", taxonomy.ListSkills)
		public.GET("/insights/skill-demand", insights.SkillDemand)
		public.GET("/insights/skill-demand/:skill_id/jobs", insights.EvidenceJobs)
		public.GET("/resources", resources.List)

		user := public.Group("", middleware.RequireSession(sessionSvc), middleware.MaxBody(defaultMaxBody))
		user.GET("/profile", profile.Get)
		user.PUT("/profile", profile.Put)
		user.GET("/gap", gap.Get)
		user.GET("/roadmap", roadmap.Get)
		// Generate memanggil LLM: batasi lebih ketat.
		user.POST("/roadmap/generate",
			middleware.PerMinute(cfg.RoadmapGeneratePerMinute).Middleware(middleware.ByIP),
			roadmap.Generate)

		admin := api.Group("/admin", middleware.RequireAdmin(cfg.AdminToken), middleware.MaxBody(adminMaxBody))
		admin.POST("/jobs", adminJobs.Create)
		admin.POST("/resources", resources.Create)
	}
	return r
}
