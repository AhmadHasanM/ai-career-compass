package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/config"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/testutil"
)

type jobSpec struct {
	role, level, status string
	skills              map[string]string // slug -> required/preferred
}

func seedJobs(t *testing.T, db *pgxpool.Pool, jobs []jobSpec) {
	t.Helper()
	ctx := context.Background()
	for i, j := range jobs {
		var id string
		err := db.QueryRow(ctx, `
			INSERT INTO job_postings (title, role_id, level, source_name, raw_text, extraction_status, collected_at)
			VALUES ($1, (SELECT id FROM roles WHERE slug = $2), $3, 'test', 'x', $4, DATE '2026-09-01' + $5::int)
			RETURNING id`, fmt.Sprintf("Job %d", i), j.role, j.level, j.status, i).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		for slug, req := range j.skills {
			if _, err := db.Exec(ctx, `
				INSERT INTO job_skills (job_id, skill_id, requirement_type)
				VALUES ($1, (SELECT id FROM skills WHERE slug = $2), $3)`, id, slug, req); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := db.Exec(ctx, `REFRESH MATERIALIZED VIEW skill_demand`); err != nil {
		t.Fatal(err)
	}
}

// Data pasar: 4 lowongan AI Engineer selesai (+1 pending dan 1 ML Engineer yang tidak boleh dihitung).
// Python 3/4 = 75% (wajib 75%), RAG 3/4 = 75% (wajib 50%), PostgreSQL 25% (wajib 25%), Docker 25% (wajib 0%).
var marketJobs = []jobSpec{
	{"ai-engineer", "mid", "done", map[string]string{"python": "required", "rag": "required", "docker": "preferred"}},
	{"ai-engineer", "mid", "done", map[string]string{"python": "required", "rag": "preferred"}},
	{"ai-engineer", "senior", "done", map[string]string{"python": "required", "postgresql": "required"}},
	{"ai-engineer", "junior", "done", map[string]string{"rag": "required"}},
	{"ai-engineer", "mid", "pending", map[string]string{"docker": "required"}},
	{"ml-engineer", "mid", "done", map[string]string{"docker": "required"}},
}

func decodeInto(t *testing.T, body map[string]any, dst any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatal(err)
	}
}

func TestSkillDemand(t *testing.T) {
	a := newAPI(t)
	seedJobs(t, testutil.DB(t), marketJobs)

	r := a.do(http.MethodGet, "/api/insights/skill-demand", nil)
	var got model.SkillDemand
	decodeInto(t, r.Body, &got)
	if r.Code != http.StatusOK || got.Role.Slug != "ai-engineer" || got.TotalJobs != 4 || !got.SmallSample {
		t.Fatalf("= %d %+v", r.Code, got)
	}
	if got.SnapshotDate == nil || got.SnapshotDate.Format("2006-01-02") != "2026-09-04" {
		t.Errorf("snapshot = %v, ingin 2026-09-04 (collected_at terbaru yang dihitung)", got.SnapshotDate)
	}
	want := []struct {
		slug        string
		demand, req float64
	}{{"python", 75, 75}, {"rag", 75, 50}, {"postgresql", 25, 25}, {"docker", 25, 0}}
	if len(got.Items) != len(want) {
		t.Fatalf("items = %+v", got.Items)
	}
	for i, w := range want {
		it := got.Items[i]
		if it.Slug != w.slug || it.DemandPct != w.demand || it.RequiredPct != w.req {
			t.Errorf("items[%d] = %s %v/%v, ingin %s %v/%v", i, it.Slug, it.DemandPct, it.RequiredPct, w.slug, w.demand, w.req)
		}
	}

	r = a.do(http.MethodGet, "/api/insights/skill-demand?level=mid&limit=2", nil)
	decodeInto(t, r.Body, &got)
	if got.TotalJobs != 2 || len(got.Items) != 2 || got.Items[0].Slug != "python" || got.Items[0].DemandPct != 100 {
		t.Errorf("filter level mid = %+v", got)
	}

	for _, q := range []string{"level=cto", "limit=0x", "limit=500", "role=astronaut"} {
		if r := a.do(http.MethodGet, "/api/insights/skill-demand?"+q, nil); r.Code/100 != 4 {
			t.Errorf("%s = %d, ingin 4xx", q, r.Code)
		}
	}
}

func TestEvidenceJobs(t *testing.T) {
	a := newAPI(t)
	seedJobs(t, testutil.DB(t), marketJobs)
	rag := testutil.SkillID(t, "rag")

	r := a.do(http.MethodGet, fmt.Sprintf("/api/insights/skill-demand/%d/jobs", rag), nil)
	var got model.EvidenceJobs
	decodeInto(t, r.Body, &got)
	if r.Code != http.StatusOK || got.Total != 3 || len(got.Jobs) != 3 || got.Skill.Slug != "rag" {
		t.Fatalf("= %d %+v", r.Code, got)
	}
	// wajib dulu, lalu terbaru
	if got.Jobs[0].RequirementType != "required" || got.Jobs[2].RequirementType != "preferred" {
		t.Errorf("urutan = %+v", got.Jobs)
	}

	r = a.do(http.MethodGet, fmt.Sprintf("/api/insights/skill-demand/%d/jobs?limit=1&offset=1", rag), nil)
	decodeInto(t, r.Body, &got)
	if got.Total != 3 || len(got.Jobs) != 1 {
		t.Errorf("paging = %+v", got)
	}

	if r := a.do(http.MethodGet, "/api/insights/skill-demand/424242/jobs", nil); r.Code != http.StatusNotFound {
		t.Errorf("skill tidak ada = %d", r.Code)
	}
	if r := a.do(http.MethodGet, "/api/insights/skill-demand/abc/jobs", nil); r.Code != http.StatusBadRequest {
		t.Errorf("skill_id bukan angka = %d", r.Code)
	}
}

// userWithProfile membuat sesi + profil AI Engineer yang sudah menguasai Docker.
func userWithProfile(t *testing.T, a *api) []string {
	t.Helper()
	h := []string{"X-Session-Id", a.newSession()}
	r := a.do(http.MethodPut, "/api/profile", map[string]any{
		"current_job":    "Backend Developer",
		"target_role_id": testutil.RoleID(t, "ai-engineer"),
		"hours_per_week": 10,
		"skills":         []map[string]any{{"skill_id": testutil.SkillID(t, "docker"), "proficiency": "intermediate"}},
	}, h...)
	if r.Code != http.StatusOK {
		t.Fatalf("PUT profile = %d %v", r.Code, r.Body)
	}
	return h
}

func TestGap(t *testing.T) {
	a := newAPI(t)
	seedJobs(t, testutil.DB(t), marketJobs)

	if r := a.do(http.MethodGet, "/api/gap", nil, "X-Session-Id", a.newSession()); r.Code != http.StatusConflict || errCode(r) != "profile_required" {
		t.Fatalf("tanpa profil = %d %v", r.Code, r.Body)
	}

	r := a.do(http.MethodGet, "/api/gap", nil, userWithProfile(t, a)...)
	var got model.Gap
	decodeInto(t, r.Body, &got)
	if r.Code != http.StatusOK || got.TotalJobs != 4 || got.ThresholdPct != 20 {
		t.Fatalf("= %d %+v", r.Code, got)
	}
	// RAG 75×1.2 = 90, Python 75×1.0 = 75, PostgreSQL 25×0.9 = 22.5; Docker dimiliki
	wantGaps := []struct {
		slug  string
		score float64
	}{{"rag", 90}, {"python", 75}, {"postgresql", 22.5}}
	if len(got.Gaps) != len(wantGaps) {
		t.Fatalf("gaps = %+v", got.Gaps)
	}
	for i, w := range wantGaps {
		if got.Gaps[i].Slug != w.slug || got.Gaps[i].PriorityScore != w.score {
			t.Errorf("gaps[%d] = %s %v, ingin %s %v", i, got.Gaps[i].Slug, got.Gaps[i].PriorityScore, w.slug, w.score)
		}
	}
	if len(got.Owned) != 1 || got.Owned[0].Slug != "docker" || got.CoveragePct != 12.5 {
		t.Errorf("owned = %+v coverage = %v", got.Owned, got.CoveragePct)
	}
}

// fakeAIService meniru ai-service: explain mengembalikan penjelasan (termasuk satu skill karangan),
// embed membalas 202.
func fakeAIService(t *testing.T, explainCalls, embedCalls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/roadmap/explain":
			explainCalls.Add(1)
			var req model.ExplainRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			var nodes []model.ExplainedNode
			for _, n := range req.Nodes {
				if n.Name == "PostgreSQL" {
					continue // node tanpa penjelasan harus jatuh ke fallback
				}
				nodes = append(nodes, model.ExplainedNode{SkillID: n.SkillID, Rationale: "LLM: " + n.Name, EstWeeks: 3})
			}
			nodes = append(nodes, model.ExplainedNode{SkillID: 999999, Rationale: "skill karangan", EstWeeks: 1})
			_ = json.NewEncoder(w).Encode(model.ExplainResponse{Model: "fake-llm", Nodes: nodes})
		case strings.HasPrefix(r.URL.Path, "/internal/resources/"):
			embedCalls.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
}

func TestRoadmapGenerateAndGet(t *testing.T) {
	var explainCalls, embedCalls atomic.Int32
	ai := fakeAIService(t, &explainCalls, &embedCalls)
	defer ai.Close()
	a := newAPI(t, func(c *config.Config) { c.AIServiceURL, c.RoadmapGeneratePerMinute = ai.URL, 100 })
	seedJobs(t, testutil.DB(t), marketJobs)
	h := userWithProfile(t, a)

	if r := a.do(http.MethodGet, "/api/roadmap", nil, h...); r.Code != http.StatusNotFound {
		t.Fatalf("GET sebelum generate = %d, ingin 404", r.Code)
	}

	// Sumber belajar untuk Python (lewat admin, memicu embed).
	res := map[string]any{"skill_slug": "python", "title": "Python Tutorial", "url": "https://docs.python.org/3/tutorial/",
		"type": "docs", "level": "beginner", "est_hours": 10}
	r := a.do(http.MethodPost, "/api/admin/resources", res, "X-Admin-Token", adminToken)
	if r.Code != http.StatusCreated || r.Body["embedding_queued"] != true {
		t.Fatalf("create resource = %d %v", r.Code, r.Body)
	}

	r = a.do(http.MethodPost, "/api/roadmap/generate", nil, h...)
	var rm model.Roadmap
	decodeInto(t, r.Body, &rm)
	if r.Code != http.StatusCreated || rm.Version != 1 || rm.ModelName == nil || *rm.ModelName != "fake-llm" {
		t.Fatalf("generate = %d %+v", r.Code, r.Body)
	}
	var order []string
	for _, n := range rm.Nodes {
		order = append(order, n.Slug)
	}
	// Python sebelum RAG (prasyarat), RAG sebelum PostgreSQL (skor lebih tinggi); Docker dimiliki
	if strings.Join(order, ",") != "python,rag,postgresql" {
		t.Fatalf("urutan = %v", order)
	}
	py, ragNode, pg := rm.Nodes[0], rm.Nodes[1], rm.Nodes[2]
	if py.Stage != "Tahap 1" || ragNode.Stage != "Tahap 2" || pg.Stage != "Tahap 1" {
		t.Errorf("stage = %s %s %s", py.Stage, ragNode.Stage, pg.Stage)
	}
	if *py.Rationale != "LLM: Python" || *py.EstWeeks != 3 {
		t.Errorf("node python = %q %v", *py.Rationale, *py.EstWeeks)
	}
	if !strings.HasPrefix(*pg.Rationale, "Diminta di 25% lowongan") || *pg.EstWeeks != 2 {
		t.Errorf("node postgres harus fallback (20 jam / 10 jam per minggu): %q %v", *pg.Rationale, *pg.EstWeeks)
	}
	if len(py.Resources) != 1 || py.Resources[0].Title != "Python Tutorial" || len(pg.Resources) != 0 {
		t.Errorf("resources = %+v / %+v", py.Resources, pg.Resources)
	}
	if len(rm.Edges) != 1 || rm.Edges[0].From != py.SkillID || rm.Edges[0].To != ragNode.SkillID {
		t.Errorf("edges = %+v", rm.Edges)
	}
	if rm.TotalWeeks != 8 {
		t.Errorf("total_weeks = %v, ingin 3+3+2", rm.TotalWeeks)
	}

	r = a.do(http.MethodPost, "/api/roadmap/generate", nil, h...)
	decodeInto(t, r.Body, &rm)
	if rm.Version != 2 {
		t.Errorf("generate ulang = versi %d, ingin 2", rm.Version)
	}
	r = a.do(http.MethodGet, "/api/roadmap", nil, h...)
	decodeInto(t, r.Body, &rm)
	if r.Code != http.StatusOK || rm.Version != 2 || len(rm.Nodes) != 3 {
		t.Errorf("GET = %d versi %d", r.Code, rm.Version)
	}
	if explainCalls.Load() != 2 || embedCalls.Load() != 1 {
		t.Errorf("panggilan explain = %d, embed = %d", explainCalls.Load(), embedCalls.Load())
	}
}

func TestRoadmapFallsBackWithoutAIService(t *testing.T) {
	a := newAPI(t, func(c *config.Config) { c.RoadmapGeneratePerMinute = 100 })
	seedJobs(t, testutil.DB(t), marketJobs)
	r := a.do(http.MethodPost, "/api/roadmap/generate", nil, userWithProfile(t, a)...)
	var rm model.Roadmap
	decodeInto(t, r.Body, &rm)
	if r.Code != http.StatusCreated || *rm.ModelName != "fallback" || len(rm.Nodes) != 3 {
		t.Fatalf("= %d %+v", r.Code, r.Body)
	}
	for _, n := range rm.Nodes {
		if n.Rationale == nil || *n.Rationale == "" || n.EstWeeks == nil {
			t.Errorf("node %s tanpa penjelasan fallback", n.Slug)
		}
	}
}

func TestRoadmapIncludesMissingPrerequisites(t *testing.T) {
	a := newAPI(t, func(c *config.Config) { c.RoadmapGeneratePerMinute = 100 })
	// Hanya RAG yang diminta pasar; Python (prasyaratnya) tidak ada di lowongan sama sekali.
	seedJobs(t, testutil.DB(t), []jobSpec{
		{"ai-engineer", "mid", "done", map[string]string{"rag": "required"}},
	})
	r := a.do(http.MethodPost, "/api/roadmap/generate", nil, userWithProfile(t, a)...)
	var rm model.Roadmap
	decodeInto(t, r.Body, &rm)
	if len(rm.Nodes) != 2 || rm.Nodes[0].Slug != "python" || rm.Nodes[1].Slug != "rag" {
		t.Fatalf("nodes = %+v", rm.Nodes)
	}
	if rm.Nodes[0].DemandPct != nil || !strings.Contains(*rm.Nodes[0].Rationale, "Prasyarat untuk Retrieval-Augmented Generation") {
		t.Errorf("node prasyarat = %+v", rm.Nodes[0])
	}
}

func TestResources(t *testing.T) {
	var explainCalls, embedCalls atomic.Int32
	ai := fakeAIService(t, &explainCalls, &embedCalls)
	defer ai.Close()
	a := newAPI(t, func(c *config.Config) { c.AIServiceURL = ai.URL })
	auth := []string{"X-Admin-Token", adminToken}
	py := testutil.SkillID(t, "python")

	res := map[string]any{"skill_id": py, "title": "Python Tutorial", "url": "https://docs.python.org/3/tutorial/", "type": "docs"}
	r := a.do(http.MethodPost, "/api/admin/resources", res, auth...)
	if r.Code != http.StatusCreated || r.Body["language"] != "en" || r.Body["is_free"] != true {
		t.Fatalf("create = %d %v", r.Code, r.Body)
	}
	id := r.Body["id"]
	if r := a.do(http.MethodPost, "/api/admin/resources", res, auth...); r.Code != http.StatusConflict || r.Body["existing_id"] != id {
		t.Errorf("duplikat = %d %v", r.Code, r.Body)
	}

	bad := map[string]any{"skill_slug": "cobol", "title": "x", "url": "ftp://x", "type": "podcast", "language": "fr", "est_hours": -1}
	r = a.do(http.MethodPost, "/api/admin/resources", bad, auth...)
	f := errFields(r)
	for _, k := range []string{"skill_slug", "title", "url", "type", "language", "est_hours"} {
		if f[k] == nil {
			t.Errorf("field %q tidak dilaporkan: %v", k, f)
		}
	}

	r = a.do(http.MethodGet, fmt.Sprintf("/api/resources?skill_id=%d", py), nil)
	if list, _ := r.Body["resources"].([]any); r.Code != http.StatusOK || len(list) != 1 {
		t.Errorf("list = %d %v", r.Code, r.Body)
	}
	if r := a.do(http.MethodGet, "/api/resources", nil); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("tanpa skill_id = %d", r.Code)
	}
}
