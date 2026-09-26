package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/middleware"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/service"
)

type SessionHandler struct{ svc *service.SessionService }

// Create: POST /api/sessions
func (h *SessionHandler) Create(c *gin.Context) {
	s, err := h.svc.Create(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, s)
}

type TaxonomyHandler struct{ svc *service.TaxonomyService }

// ListRoles: GET /api/roles
func (h *TaxonomyHandler) ListRoles(c *gin.Context) {
	roles, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

// ListSkills: GET /api/skills?category=llm
func (h *TaxonomyHandler) ListSkills(c *gin.Context) {
	skills, err := h.svc.ListSkills(c.Request.Context(), c.Query("category"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills})
}

type ProfileHandler struct{ svc *service.ProfileService }

// Get: GET /api/profile
func (h *ProfileHandler) Get(c *gin.Context) {
	p, err := h.svc.Get(c.Request.Context(), middleware.SessionID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// Put: PUT /api/profile
func (h *ProfileHandler) Put(c *gin.Context) {
	var req service.ProfileRequest
	if !decodeJSON(c, &req) {
		return
	}
	p, err := h.svc.Save(c.Request.Context(), middleware.SessionID(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

type AdminJobHandler struct{ svc *service.JobService }

// Create: POST /api/admin/jobs
func (h *AdminJobHandler) Create(c *gin.Context) {
	var req service.JobRequest
	if !decodeJSON(c, &req) {
		return
	}
	job, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, job)
}
