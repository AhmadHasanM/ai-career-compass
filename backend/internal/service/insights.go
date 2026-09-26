package service

import (
	"context"
	"errors"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

const (
	// MVP hanya satu role; filter role aktif setelah role kedua punya data.
	DefaultRoleSlug      = "ai-engineer"
	smallSampleThreshold = 30
	defaultDemandLimit   = 15
	maxListLimit         = 100
)

type InsightsService struct {
	insights *repository.InsightsRepository
}

func NewInsightsService(insights *repository.InsightsRepository) *InsightsService {
	return &InsightsService{insights: insights}
}

func (s *InsightsService) role(ctx context.Context, slug string) (model.Role, error) {
	if slug == "" {
		slug = DefaultRoleSlug
	}
	role, err := s.insights.RoleBySlug(ctx, slug)
	if errors.Is(err, repository.ErrNotFound) {
		return role, &ValidationError{Fields: map[string]string{"role": "role tidak ditemukan"}}
	}
	return role, err
}

func parseLevel(level string) (*string, error) {
	if level == "" {
		return nil, nil
	}
	if !slices.Contains(model.JobLevels, level) {
		return nil, &ValidationError{Fields: map[string]string{"level": "harus salah satu dari: " + strings.Join(model.JobLevels, ", ")}}
	}
	return &level, nil
}

func (s *InsightsService) sample(ctx context.Context, roleID int16, level *string) (model.MarketSample, error) {
	n, snapshot, err := s.insights.Sample(ctx, roleID, level)
	return model.MarketSample{TotalJobs: n, SnapshotDate: snapshot, SmallSample: n < smallSampleThreshold}, err
}

// SkillDemand: top skill untuk role (default AI Engineer), opsional filter level.
func (s *InsightsService) SkillDemand(ctx context.Context, roleSlug, level string, limit int) (*model.SkillDemand, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = defaultDemandLimit
	}
	if limit < 1 || limit > maxListLimit {
		return nil, &ValidationError{Fields: map[string]string{"limit": "harus antara 1 dan 100"}}
	}
	role, err := s.role(ctx, roleSlug)
	if err != nil {
		return nil, err
	}
	sample, err := s.sample(ctx, role.ID, lvl)
	if err != nil {
		return nil, err
	}
	items, err := s.insights.Demand(ctx, role.ID, lvl, limit)
	if err != nil {
		return nil, err
	}
	return &model.SkillDemand{Role: role, Level: lvl, MarketSample: sample, Items: items}, nil
}

// EvidenceJobs: daftar lowongan asal sebuah persentase.
func (s *InsightsService) EvidenceJobs(ctx context.Context, skillID int, roleSlug, level string, limit, offset int) (*model.EvidenceJobs, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > maxListLimit || offset < 0 {
		return nil, &ValidationError{Fields: map[string]string{"limit": "limit 1–100, offset ≥ 0"}}
	}
	role, err := s.role(ctx, roleSlug)
	if err != nil {
		return nil, err
	}
	skill, err := s.insights.Skill(ctx, skillID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	skill.Aliases, skill.PrerequisiteIDs = nil, nil
	jobs, total, err := s.insights.EvidenceJobs(ctx, skillID, role.ID, lvl, limit, offset)
	if err != nil {
		return nil, err
	}
	return &model.EvidenceJobs{Skill: skill, Total: total, Jobs: jobs}, nil
}

// GapConfig: skill dianggap gap jika demand_pct >= ThresholdPct dan belum dimiliki.
// Skor prioritas = demand_pct × bobot kategori (Alur 3 dokumen perencanaan).
type GapConfig struct {
	ThresholdPct    float64
	CategoryWeights map[string]float64
}

// DefaultGapConfig: bobot awal menonjolkan skill inti AI Engineer (llm, ml); bisa disetel ulang
// setelah data lowongan terkumpul.
var DefaultGapConfig = GapConfig{
	ThresholdPct: 20,
	CategoryWeights: map[string]float64{
		"llm": 1.2, "ml": 1.1, "language": 1.0, "framework": 1.0, "mlops": 1.0, "data": 0.9, "cloud": 0.9,
	},
}

func (c GapConfig) weight(category string) float64 {
	if w, ok := c.CategoryWeights[category]; ok {
		return w
	}
	return 1
}

// ComputeGap memisahkan skill di atas ambang menjadi gap (belum dimiliki, urut prioritas)
// dan owned (sudah dimiliki), plus persentase cakupan permintaan yang sudah dikuasai.
func ComputeGap(demand []model.DemandItem, owned map[int]bool, cfg GapConfig) (gaps []model.GapItem, have []model.DemandItem, coveragePct float64) {
	var totalDemand, ownedDemand float64
	gaps, have = []model.GapItem{}, []model.DemandItem{}
	for _, d := range demand {
		if d.DemandPct < cfg.ThresholdPct {
			continue
		}
		totalDemand += d.DemandPct
		if owned[d.SkillID] {
			ownedDemand += d.DemandPct
			have = append(have, d)
			continue
		}
		gaps = append(gaps, model.GapItem{DemandItem: d, PriorityScore: round2(d.DemandPct * cfg.weight(d.Category))})
	}
	sort.SliceStable(gaps, func(i, j int) bool {
		a, b := gaps[i], gaps[j]
		if a.PriorityScore != b.PriorityScore {
			return a.PriorityScore > b.PriorityScore
		}
		if a.RequiredPct != b.RequiredPct {
			return a.RequiredPct > b.RequiredPct
		}
		return a.Name < b.Name
	})
	if totalDemand > 0 {
		coveragePct = round2(100 * ownedDemand / totalDemand)
	}
	return gaps, have, coveragePct
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// GapService menghitung gap skill pengguna terhadap role target di profilnya.
type GapService struct {
	insights *repository.InsightsRepository
	profiles *repository.ProfileRepository
	cfg      GapConfig
}

func NewGapService(insights *repository.InsightsRepository, profiles *repository.ProfileRepository, cfg GapConfig) *GapService {
	return &GapService{insights: insights, profiles: profiles, cfg: cfg}
}

// ErrProfileIncomplete: profil belum ada atau belum punya target role.
var ErrProfileIncomplete = errors.New("profil belum lengkap")

type gapInput struct {
	profile *model.Profile
	role    model.Role
	sample  model.MarketSample
	demand  []model.DemandItem
	owned   map[int]bool
}

func (s *GapService) load(ctx context.Context, sessionID uuid.UUID) (*gapInput, error) {
	p, err := s.profiles.GetBySession(ctx, sessionID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrProfileIncomplete
	}
	if err != nil {
		return nil, err
	}
	if p.TargetRole == nil {
		return nil, ErrProfileIncomplete
	}
	n, snapshot, err := s.insights.Sample(ctx, p.TargetRole.ID, nil)
	if err != nil {
		return nil, err
	}
	demand, err := s.insights.Demand(ctx, p.TargetRole.ID, nil, 0)
	if err != nil {
		return nil, err
	}
	owned := make(map[int]bool, len(p.Skills))
	for _, sk := range p.Skills {
		owned[sk.SkillID] = true
	}
	return &gapInput{
		profile: p,
		role:    *p.TargetRole,
		sample:  model.MarketSample{TotalJobs: n, SnapshotDate: snapshot, SmallSample: n < smallSampleThreshold},
		demand:  demand,
		owned:   owned,
	}, nil
}

func (s *GapService) Get(ctx context.Context, sessionID uuid.UUID) (*model.Gap, error) {
	in, err := s.load(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	gaps, have, coverage := ComputeGap(in.demand, in.owned, s.cfg)
	return &model.Gap{
		Role:         in.role,
		MarketSample: in.sample,
		ThresholdPct: s.cfg.ThresholdPct,
		CoveragePct:  coverage,
		Gaps:         gaps,
		Owned:        have,
	}, nil
}
