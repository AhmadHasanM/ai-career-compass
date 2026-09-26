package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

const (
	maxRoadmapTargets  = 15 // gap teratas yang dijadikan target; prasyarat bisa menambah node
	maxRoadmapNodes    = 20 // batas total node agar roadmap tetap bisa dijalani
	resourcesPerNode   = 3
	explainTimeout     = 20 * time.Second
	maxRationaleChars  = 500
	minEstWeeks        = 0.5
	maxEstWeeks        = 12
	fallbackSkillHours = 20 // estimasi jam belajar per skill jika LLM tidak tersedia
	fallbackModelName  = "fallback"
)

// RoadmapExplainer meminta LLM (ai-service) menjelaskan urutan roadmap; tidak boleh menambah skill.
type RoadmapExplainer interface {
	ExplainRoadmap(ctx context.Context, req model.ExplainRequest) (*model.ExplainResponse, error)
}

type RoadmapService struct {
	gap       *GapService
	roadmaps  *repository.RoadmapRepository
	resources *repository.ResourceRepository
	explainer RoadmapExplainer // nil = selalu pakai penjelasan fallback
	log       *slog.Logger
}

func NewRoadmapService(gap *GapService, roadmaps *repository.RoadmapRepository, resources *repository.ResourceRepository,
	explainer RoadmapExplainer, log *slog.Logger) *RoadmapService {
	return &RoadmapService{gap: gap, roadmaps: roadmaps, resources: resources, explainer: explainer, log: log}
}

// draftNode: node roadmap sebelum disimpan.
type draftNode struct {
	skill       repository.SkillInfo
	depth       int
	priority    *float64
	demandPct   *float64
	requiredFor []string
	estWeeks    float64
	rationale   string
}

func stageName(depth int) string { return fmt.Sprintf("Tahap %d", depth+1) }

// Generate menyusun roadmap baru dari gap pengguna. Urutan ditentukan Go (topological sort),
// LLM hanya memberi alasan dan estimasi durasi per node.
func (s *RoadmapService) Generate(ctx context.Context, sessionID uuid.UUID) (*model.Roadmap, error) {
	in, err := s.gap.load(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	gaps, _, _ := ComputeGap(in.demand, in.owned, s.gap.cfg)
	if len(gaps) > maxRoadmapTargets {
		gaps = gaps[:maxRoadmapTargets]
	}

	prereqs, err := s.roadmaps.Prerequisites(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]int, len(gaps))
	priority := map[int]float64{}
	for i, g := range gaps {
		targets[i] = g.SkillID
		priority[g.SkillID] = g.PriorityScore
	}
	nodeIDs := CappedClosure(targets, prereqs, in.owned, maxRoadmapNodes)

	skills, err := s.roadmaps.SkillsByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	// Prasyarat tanpa skor gap diurutkan setelah skill bernilai tinggi yang sama-sama siap,
	// tetapi topological sort tetap memaksanya muncul sebelum skill yang membutuhkannya.
	before := func(a, b int) bool {
		if priority[a] != priority[b] {
			return priority[a] > priority[b]
		}
		return skills[a].Name < skills[b].Name
	}
	order, depth, err := TopoSort(nodeIDs, prereqs, before)
	if err != nil {
		return nil, err
	}

	demandPct := map[int]float64{}
	for _, d := range in.demand {
		demandPct[d.SkillID] = d.DemandPct
	}
	inRoadmap := map[int]bool{}
	for _, id := range order {
		inRoadmap[id] = true
	}
	hours := 5
	if in.profile.HoursPerWeek != nil {
		hours = int(*in.profile.HoursPerWeek)
	}

	drafts := make([]*draftNode, len(order))
	for i, id := range order {
		d := &draftNode{skill: skills[id], depth: depth[id]}
		if p, ok := priority[id]; ok {
			d.priority = &p
		}
		if pct, ok := demandPct[id]; ok {
			d.demandPct = &pct
		}
		for dep, ps := range prereqs {
			for _, p := range ps {
				if p == id && inRoadmap[dep] {
					d.requiredFor = append(d.requiredFor, skills[dep].Name)
				}
			}
		}
		sort.Strings(d.requiredFor)
		d.rationale = fallbackRationale(d, in.role.Name)
		d.estWeeks = fallbackWeeks(hours)
		drafts[i] = d
	}

	modelName := fallbackModelName
	if s.explainer != nil && len(drafts) > 0 {
		modelName = s.explain(ctx, in, drafts, prereqs, skills, inRoadmap, hours)
	}

	inputs := make([]model.RoadmapNodeInput, len(drafts))
	for i, d := range drafts {
		inputs[i] = model.RoadmapNodeInput{
			SkillID: d.skill.ID, OrderIndex: i, Stage: stageName(d.depth),
			PriorityScore: d.priority, DemandPct: d.demandPct, EstWeeks: d.estWeeks, Rationale: d.rationale,
		}
	}
	if _, err := s.roadmaps.Save(ctx, in.profile.ID, in.role.ID, modelName, inputs); err != nil {
		return nil, err
	}
	return s.latest(ctx, in.profile)
}

// explain memanggil LLM; kegagalan apa pun membuat roadmap tetap jadi dengan penjelasan fallback.
func (s *RoadmapService) explain(ctx context.Context, in *gapInput, drafts []*draftNode, prereqs map[int][]int,
	skills map[int]repository.SkillInfo, inRoadmap map[int]bool, hours int) string {
	req := model.ExplainRequest{
		Role: in.role.Name,
		Profile: model.ExplainProfile{
			Education: in.profile.Education, CurrentJob: in.profile.CurrentJob, HoursPerWeek: hours,
			Skills: make([]string, 0, len(in.profile.Skills)),
		},
	}
	for _, sk := range in.profile.Skills {
		req.Profile.Skills = append(req.Profile.Skills, sk.Name)
	}
	for _, d := range drafts {
		var pre []string
		for _, p := range prereqs[d.skill.ID] {
			if inRoadmap[p] {
				pre = append(pre, skills[p].Name)
			}
		}
		sort.Strings(pre)
		req.Nodes = append(req.Nodes, model.ExplainNode{
			SkillID: d.skill.ID, Name: d.skill.Name, Category: d.skill.Category, Stage: stageName(d.depth),
			DemandPct: d.demandPct, Prerequisites: pre, RequiredFor: d.requiredFor,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, explainTimeout)
	defer cancel()
	resp, err := s.explainer.ExplainRoadmap(ctx, req)
	if err != nil {
		s.log.Warn("penjelasan roadmap gagal; memakai fallback", "error", err)
		return fallbackModelName
	}
	applied, dropped := ApplyExplanations(drafts, resp.Nodes)
	if len(dropped) > 0 {
		s.log.Warn("LLM mengembalikan skill di luar roadmap; diabaikan", "skill_ids", dropped)
	}
	if applied == 0 {
		return fallbackModelName
	}
	return resp.Model
}

// ApplyExplanations memasang alasan + estimasi dari LLM ke node yang cocok.
// Skill yang tidak ada di input ditolak (dikembalikan di dropped); duplikat diabaikan;
// node tanpa penjelasan valid tetap memakai fallback.
func ApplyExplanations(drafts []*draftNode, explained []model.ExplainedNode) (applied int, dropped []int) {
	byID := make(map[int]*draftNode, len(drafts))
	for _, d := range drafts {
		byID[d.skill.ID] = d
	}
	done := map[int]bool{}
	for _, e := range explained {
		d, ok := byID[e.SkillID]
		if !ok {
			dropped = append(dropped, e.SkillID)
			continue
		}
		if done[e.SkillID] {
			continue
		}
		done[e.SkillID] = true
		if r := strings.TrimSpace(e.Rationale); r != "" {
			d.rationale = truncateRunes(r, maxRationaleChars)
		}
		if e.EstWeeks > 0 && !math.IsInf(e.EstWeeks, 0) && !math.IsNaN(e.EstWeeks) {
			d.estWeeks = clampWeeks(e.EstWeeks)
		}
		applied++
	}
	return applied, dropped
}

func fallbackRationale(d *draftNode, roleName string) string {
	switch {
	case d.demandPct != nil && len(d.requiredFor) > 0:
		return fmt.Sprintf("Diminta di %.0f%% lowongan %s dan menjadi prasyarat %s.", *d.demandPct, roleName, strings.Join(d.requiredFor, ", "))
	case d.demandPct != nil:
		return fmt.Sprintf("Diminta di %.0f%% lowongan %s.", *d.demandPct, roleName)
	case len(d.requiredFor) > 0:
		return fmt.Sprintf("Prasyarat untuk %s.", strings.Join(d.requiredFor, ", "))
	default:
		return "Bagian dari jalur belajar " + roleName + "."
	}
}

func fallbackWeeks(hoursPerWeek int) float64 {
	if hoursPerWeek <= 0 {
		hoursPerWeek = 5
	}
	return clampWeeks(float64(fallbackSkillHours) / float64(hoursPerWeek))
}

// clampWeeks membatasi estimasi ke [0.5, 12] minggu, dibulatkan ke 0.5 terdekat.
func clampWeeks(w float64) float64 {
	w = math.Round(w*2) / 2
	return math.Max(minEstWeeks, math.Min(maxEstWeeks, w))
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// Get mengembalikan roadmap terbaru untuk target role di profil pengguna.
func (s *RoadmapService) Get(ctx context.Context, sessionID uuid.UUID) (*model.Roadmap, error) {
	p, err := s.gap.profiles.GetBySession(ctx, sessionID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrProfileIncomplete
	}
	if err != nil {
		return nil, err
	}
	if p.TargetRole == nil {
		return nil, ErrProfileIncomplete
	}
	return s.latest(ctx, p)
}

func (s *RoadmapService) latest(ctx context.Context, p *model.Profile) (*model.Roadmap, error) {
	rm, err := s.roadmaps.Latest(ctx, p.ID, p.TargetRole.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rm.HoursPerWeek = p.HoursPerWeek

	ids := make([]int, len(rm.Nodes))
	for i, n := range rm.Nodes {
		ids[i] = n.SkillID
		if n.EstWeeks != nil {
			rm.TotalWeeks += *n.EstWeeks
		}
	}
	res, err := s.resources.BySkills(ctx, ids, resourcesPerNode)
	if err != nil {
		return nil, err
	}
	for i := range rm.Nodes {
		if r, ok := res[rm.Nodes[i].SkillID]; ok {
			rm.Nodes[i].Resources = r
		}
	}
	if rm.Edges == nil {
		rm.Edges = []model.RoadmapEdge{}
	}
	return rm, nil
}
