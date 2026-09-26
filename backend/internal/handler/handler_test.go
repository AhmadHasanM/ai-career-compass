package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/config"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/handler"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/testutil"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(testutil.RunWithDB(m))
}

const adminToken = "test-admin-token"

type api struct {
	t *testing.T
	r *gin.Engine
}

func newAPI(t *testing.T, mutate ...func(*config.Config)) *api {
	cfg := &config.Config{
		AdminToken:             adminToken,
		CORSOrigins:            []string{"http://localhost:3000"},
		RateLimitRPS:           1000,
		RateLimitBurst:         1000,
		SessionCreatePerMinute: 1000,
	}
	for _, m := range mutate {
		m(cfg)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &api{t: t, r: handler.NewRouter(cfg, testutil.DB(t), log)}
}

type resp struct {
	Code   int
	Header http.Header
	Body   map[string]any
}

func (a *api) do(method, path string, body any, headers ...string) resp {
	a.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)

	out := resp{Code: w.Code, Header: w.Header()}
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out.Body); err != nil {
			a.t.Fatalf("%s %s: body bukan JSON objek: %s", method, path, w.Body.String())
		}
	}
	return out
}

func (a *api) newSession() string {
	a.t.Helper()
	r := a.do(http.MethodPost, "/api/sessions", nil)
	if r.Code != http.StatusCreated {
		a.t.Fatalf("POST /api/sessions = %d %v", r.Code, r.Body)
	}
	return r.Body["session_id"].(string)
}

func errCode(r resp) string {
	e, _ := r.Body["error"].(map[string]any)
	s, _ := e["code"].(string)
	return s
}

func errFields(r resp) map[string]any {
	e, _ := r.Body["error"].(map[string]any)
	f, _ := e["fields"].(map[string]any)
	return f
}

func TestHealth(t *testing.T) {
	a := newAPI(t)
	r := a.do(http.MethodGet, "/health", nil)
	if r.Code != http.StatusOK || r.Body["database"] != "ok" {
		t.Fatalf("health = %d %v", r.Code, r.Body)
	}
	if r.Header.Get("X-Request-Id") == "" {
		t.Error("X-Request-Id tidak di-set")
	}
}

func TestRolesAndSkills(t *testing.T) {
	a := newAPI(t)

	r := a.do(http.MethodGet, "/api/roles", nil)
	if roles, _ := r.Body["roles"].([]any); r.Code != http.StatusOK || len(roles) != 2 {
		t.Fatalf("roles = %d %v", r.Code, r.Body)
	}

	r = a.do(http.MethodGet, "/api/skills?category=mlops", nil)
	skills, _ := r.Body["skills"].([]any)
	if r.Code != http.StatusOK || len(skills) != 1 || skills[0].(map[string]any)["slug"] != "docker" {
		t.Fatalf("skills mlops = %d %v", r.Code, r.Body)
	}

	r = a.do(http.MethodGet, "/api/skills?category=blockchain", nil)
	if r.Code != http.StatusUnprocessableEntity || errFields(r)["category"] == nil {
		t.Fatalf("kategori tidak valid = %d %v", r.Code, r.Body)
	}
}

func TestProfileRequiresValidSession(t *testing.T) {
	a := newAPI(t)
	cases := []struct {
		name, header, code string
	}{
		{"tanpa header", "", "session_required"},
		{"bukan uuid", "abc", "session_invalid"},
		{"sesi tidak ada", uuid.NewString(), "session_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var h []string
			if tc.header != "" {
				h = []string{"X-Session-Id", tc.header}
			}
			r := a.do(http.MethodGet, "/api/profile", nil, h...)
			if r.Code != http.StatusUnauthorized || errCode(r) != tc.code {
				t.Errorf("= %d %v, ingin 401 %s", r.Code, r.Body, tc.code)
			}
		})
	}
}

func TestProfileLifecycle(t *testing.T) {
	a := newAPI(t)
	sid := a.newSession()
	h := []string{"X-Session-Id", sid}

	r := a.do(http.MethodGet, "/api/profile", nil, h...)
	if r.Code != http.StatusNotFound {
		t.Fatalf("GET sebelum PUT = %d, ingin 404", r.Code)
	}

	py, rag := testutil.SkillID(t, "python"), testutil.SkillID(t, "rag")
	body := map[string]any{
		"education":      "  S1 Informatika ",
		"current_job":    "Backend Developer",
		"target_role_id": testutil.RoleID(t, "ai-engineer"),
		"hours_per_week": 10,
		"skills": []map[string]any{
			{"skill_id": py, "proficiency": "intermediate"},
			{"skill_id": rag},
		},
	}
	r = a.do(http.MethodPut, "/api/profile", body, h...)
	if r.Code != http.StatusOK {
		t.Fatalf("PUT = %d %v", r.Code, r.Body)
	}
	if r.Body["education"] != "S1 Informatika" {
		t.Errorf("education tidak di-trim: %v", r.Body["education"])
	}
	skills := r.Body["skills"].([]any)
	if len(skills) != 2 {
		t.Fatalf("skills = %v", skills)
	}
	for _, s := range skills {
		m := s.(map[string]any)
		if m["slug"] == "rag" && m["proficiency"] != "beginner" {
			t.Errorf("proficiency default seharusnya beginner, dapat %v", m["proficiency"])
		}
	}

	r = a.do(http.MethodGet, "/api/profile", nil, h...)
	role, _ := r.Body["target_role"].(map[string]any)
	if r.Code != http.StatusOK || role["slug"] != "ai-engineer" || len(r.Body["skills"].([]any)) != 2 {
		t.Fatalf("GET setelah PUT = %d %v", r.Code, r.Body)
	}

	// Profil sesi lain terpisah.
	other := a.do(http.MethodGet, "/api/profile", nil, "X-Session-Id", a.newSession())
	if other.Code != http.StatusNotFound {
		t.Errorf("sesi lain melihat profil orang lain: %d", other.Code)
	}
}

func TestProfileValidation(t *testing.T) {
	a := newAPI(t)
	h := []string{"X-Session-Id", a.newSession()}
	py := testutil.SkillID(t, "python")

	r := a.do(http.MethodPut, "/api/profile", map[string]any{
		"target_role_id": 999,
		"hours_per_week": 200,
		"skills": []map[string]any{
			{"skill_id": py, "proficiency": "expert"},
			{"skill_id": py},
		},
	}, h...)
	if r.Code != http.StatusUnprocessableEntity {
		t.Fatalf("= %d %v, ingin 422", r.Code, r.Body)
	}
	f := errFields(r)
	for _, k := range []string{"target_role_id", "hours_per_week", "skills[0].proficiency", "skills[1].skill_id"} {
		if f[k] == nil {
			t.Errorf("field %q tidak dilaporkan; fields = %v", k, f)
		}
	}

	r = a.do(http.MethodPut, "/api/profile", map[string]any{
		"target_role_id": testutil.RoleID(t, "ai-engineer"),
		"hours_per_week": 5,
		"skills":         []map[string]any{{"skill_id": 424242}},
	}, h...)
	if r.Code != http.StatusUnprocessableEntity || errFields(r)["skills"] == nil {
		t.Errorf("skill tidak ada = %d %v", r.Code, r.Body)
	}

	r = a.do(http.MethodPut, "/api/profile", `{"hours_per_week": 5, "umur": 20}`, h...)
	if r.Code != http.StatusBadRequest || errCode(r) != "invalid_json" {
		t.Errorf("field tak dikenal = %d %v", r.Code, r.Body)
	}

	r = a.do(http.MethodPut, "/api/profile", `{"hours_per_week": "sepuluh"}`, h...)
	if r.Code != http.StatusBadRequest || errFields(r)["hours_per_week"] == nil {
		t.Errorf("tipe salah = %d %v", r.Code, r.Body)
	}

	r = a.do(http.MethodPut, "/api/profile", `{"education": "`+strings.Repeat("x", 70<<10)+`"}`, h...)
	if r.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("body besar = %d, ingin 413", r.Code)
	}
}

func TestAdminCreateJob(t *testing.T) {
	a := newAPI(t)
	job := map[string]any{
		"title":        "AI Engineer",
		"company":      "PT Contoh",
		"source_name":  "JobStreet",
		"source_url":   "https://id.jobstreet.com/id/job/1",
		"collected_at": "2026-09-20",
		"work_type":    "hybrid",
		"raw_text":     testutil.LongText(),
	}

	if r := a.do(http.MethodPost, "/api/admin/jobs", job); r.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa token = %d, ingin 401", r.Code)
	}

	auth := []string{"X-Admin-Token", adminToken}
	r := a.do(http.MethodPost, "/api/admin/jobs", job, auth...)
	if r.Code != http.StatusCreated || r.Body["extraction_status"] != "pending" {
		t.Fatalf("create = %d %v", r.Code, r.Body)
	}
	id := r.Body["id"]

	r = a.do(http.MethodPost, "/api/admin/jobs", job, auth...)
	if r.Code != http.StatusConflict || r.Body["existing_id"] != id {
		t.Fatalf("duplikat = %d %v, ingin 409 existing_id %v", r.Code, r.Body, id)
	}

	job["source_url"], job["raw_text"], job["level"] = "ftp://x", "pendek", "boss"
	r = a.do(http.MethodPost, "/api/admin/jobs", job, auth...)
	f := errFields(r)
	if r.Code != http.StatusUnprocessableEntity || f["source_url"] == nil || f["raw_text"] == nil || f["level"] == nil {
		t.Fatalf("invalid = %d %v", r.Code, r.Body)
	}
}

func TestAdminCreateJobTriggersProcessing(t *testing.T) {
	var gotPath, gotToken string
	fakeAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotToken = r.URL.Path, r.Header.Get("X-Internal-Token")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer fakeAI.Close()

	a := newAPI(t, func(c *config.Config) { c.AIServiceURL, c.InternalToken = fakeAI.URL, "internal-secret" })
	job := map[string]any{"title": "AI Engineer", "source_name": "Glints", "raw_text": testutil.LongText()}
	r := a.do(http.MethodPost, "/api/admin/jobs", job, "X-Admin-Token", adminToken)
	if r.Code != http.StatusCreated || r.Body["processing_queued"] != true {
		t.Fatalf("create = %d %v", r.Code, r.Body)
	}
	if want := fmt.Sprintf("/internal/jobs/%s/process", r.Body["id"]); gotPath != want || gotToken != "internal-secret" {
		t.Errorf("ai-service dipanggil %q token %q; ingin %q token internal-secret", gotPath, gotToken, want)
	}

	// ai-service mati: lowongan tetap tersimpan, processing_queued false.
	fakeAI.Close()
	r = a.do(http.MethodPost, "/api/admin/jobs", job, "X-Admin-Token", adminToken)
	if r.Code != http.StatusCreated || r.Body["processing_queued"] != false || r.Body["extraction_status"] != "pending" {
		t.Fatalf("ai-service mati = %d %v", r.Code, r.Body)
	}
}

func TestCORS(t *testing.T) {
	a := newAPI(t)

	r := a.do(http.MethodOptions, "/api/profile", nil,
		"Origin", "http://localhost:3000", "Access-Control-Request-Method", "PUT")
	if r.Code != http.StatusNoContent ||
		r.Header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" ||
		!strings.Contains(r.Header.Get("Access-Control-Allow-Headers"), "X-Session-Id") {
		t.Fatalf("preflight = %d %v", r.Code, r.Header)
	}

	r = a.do(http.MethodGet, "/api/roles", nil, "Origin", "https://evil.example")
	if r.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("origin asing tidak boleh diizinkan")
	}
}

func TestRateLimit(t *testing.T) {
	a := newAPI(t, func(c *config.Config) { c.SessionCreatePerMinute = 2 })
	var codes []int
	for i := 0; i < 3; i++ {
		codes = append(codes, a.do(http.MethodPost, "/api/sessions", nil).Code)
	}
	if fmt.Sprint(codes) != "[201 201 429]" {
		t.Errorf("codes = %v, ingin [201 201 429]", codes)
	}
}

func TestUnknownRouteAndMethod(t *testing.T) {
	a := newAPI(t)
	if r := a.do(http.MethodGet, "/api/tidak-ada", nil); r.Code != http.StatusNotFound || errCode(r) != "not_found" {
		t.Errorf("404 = %d %v", r.Code, r.Body)
	}
	if r := a.do(http.MethodDelete, "/api/roles", nil); r.Code != http.StatusMethodNotAllowed {
		t.Errorf("405 = %d %v", r.Code, r.Body)
	}
}
