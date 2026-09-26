package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/httpx"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/middleware"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/service"
)

// queryInt membaca parameter query bilangan bulat; kosong = 0. Menulis 400 jika tidak valid.
func queryInt(c *gin.Context, name string) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		httpx.AbortValidation(c, http.StatusBadRequest, "parameter tidak valid", map[string]string{name: "harus bilangan bulat"})
		return 0, false
	}
	return n, true
}

type InsightsHandler struct{ svc *service.InsightsService }

// SkillDemand: GET /api/insights/skill-demand?role=&level=&limit=
func (h *InsightsHandler) SkillDemand(c *gin.Context) {
	limit, ok := queryInt(c, "limit")
	if !ok {
		return
	}
	out, err := h.svc.SkillDemand(c.Request.Context(), c.Query("role"), c.Query("level"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// EvidenceJobs: GET /api/insights/skill-demand/:skill_id/jobs?role=&level=&limit=&offset=
func (h *InsightsHandler) EvidenceJobs(c *gin.Context) {
	skillID, err := strconv.Atoi(c.Param("skill_id"))
	if err != nil || skillID <= 0 {
		httpx.AbortValidation(c, http.StatusBadRequest, "parameter tidak valid", map[string]string{"skill_id": "harus bilangan bulat positif"})
		return
	}
	limit, ok := queryInt(c, "limit")
	if !ok {
		return
	}
	offset, ok := queryInt(c, "offset")
	if !ok {
		return
	}
	out, err := h.svc.EvidenceJobs(c.Request.Context(), skillID, c.Query("role"), c.Query("level"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type GapHandler struct{ svc *service.GapService }

// Get: GET /api/gap
func (h *GapHandler) Get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), middleware.SessionID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type RoadmapHandler struct{ svc *service.RoadmapService }

// Generate: POST /api/roadmap/generate
func (h *RoadmapHandler) Generate(c *gin.Context) {
	out, err := h.svc.Generate(c.Request.Context(), middleware.SessionID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// Get: GET /api/roadmap
func (h *RoadmapHandler) Get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), middleware.SessionID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type ResourceHandler struct{ svc *service.ResourceService }

// List: GET /api/resources?skill_id=
func (h *ResourceHandler) List(c *gin.Context) {
	skillID, ok := queryInt(c, "skill_id")
	if !ok {
		return
	}
	out, err := h.svc.ListBySkill(c.Request.Context(), skillID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"resources": out})
}

// Create: POST /api/admin/resources
func (h *ResourceHandler) Create(c *gin.Context) {
	var req service.ResourceRequest
	if !decodeJSON(c, &req) {
		return
	}
	out, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
