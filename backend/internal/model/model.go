package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID           uuid.UUID `json:"session_id"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
}

type Role struct {
	ID   int16  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Skill struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Slug            string   `json:"slug"`
	Category        string   `json:"category"`
	Description     *string  `json:"description"`
	Aliases         []string `json:"aliases"`
	PrerequisiteIDs []int    `json:"prerequisite_ids"`
}

var SkillCategories = []string{"language", "framework", "llm", "ml", "mlops", "cloud", "data"}

var Proficiencies = []string{"beginner", "intermediate", "advanced"}

type Profile struct {
	ID           uuid.UUID   `json:"id"`
	Education    *string     `json:"education"`
	CurrentJob   *string     `json:"current_job"`
	TargetRole   *Role       `json:"target_role"`
	HoursPerWeek *int16      `json:"hours_per_week"`
	Skills       []UserSkill `json:"skills"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type UserSkill struct {
	SkillID     int    `json:"skill_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Category    string `json:"category"`
	Proficiency string `json:"proficiency"`
	Source      string `json:"source"`
}

// ProfileInput adalah isi PUT /api/profile setelah divalidasi service.
type ProfileInput struct {
	Education    *string
	CurrentJob   *string
	TargetRoleID *int16
	HoursPerWeek *int16
	Skills       []SkillInput
}

type SkillInput struct {
	SkillID     int
	Proficiency string
}

var (
	JobLevels    = []string{"intern", "junior", "mid", "senior", "lead"}
	JobWorkTypes = []string{"onsite", "remote", "hybrid"}
)

// JobInput adalah lowongan baru dari POST /api/admin/jobs setelah divalidasi service.
type JobInput struct {
	Title       string
	Company     *string
	Level       *string
	Location    *string
	WorkType    *string
	SourceName  string
	SourceURL   *string
	PostedDate  *time.Time
	CollectedAt time.Time
	RawText     string
}

type JobPosting struct {
	ID               uuid.UUID `json:"id"`
	Title            string    `json:"title"`
	Company          *string   `json:"company"`
	SourceName       string    `json:"source_name"`
	SourceURL        *string   `json:"source_url"`
	ExtractionStatus string    `json:"extraction_status"`
	CreatedAt        time.Time `json:"created_at"`
}

// DemandItem: statistik permintaan satu skill untuk satu role.
type DemandItem struct {
	SkillID       int     `json:"skill_id"`
	Name          string  `json:"name"`
	Slug          string  `json:"slug"`
	Category      string  `json:"category"`
	JobCount      int     `json:"job_count"`
	RequiredCount int     `json:"required_count"`
	DemandPct     float64 `json:"demand_pct"`
	RequiredPct   float64 `json:"required_pct"`
}

// MarketSample menjelaskan sampel lowongan di balik angka: n dan tanggal snapshot.
type MarketSample struct {
	TotalJobs    int        `json:"total_jobs"`
	SnapshotDate *time.Time `json:"snapshot_date"`
	// SmallSample true jika n < 30: persentase perlu dibaca hati-hati.
	SmallSample bool `json:"small_sample"`
}

type SkillDemand struct {
	Role  Role    `json:"role"`
	Level *string `json:"level"`
	MarketSample
	Items []DemandItem `json:"items"`
}

type EvidenceJob struct {
	ID              uuid.UUID  `json:"id"`
	Title           string     `json:"title"`
	Company         *string    `json:"company"`
	Level           *string    `json:"level"`
	Location        *string    `json:"location"`
	WorkType        *string    `json:"work_type"`
	SourceName      string     `json:"source_name"`
	SourceURL       *string    `json:"source_url"`
	PostedDate      *time.Time `json:"posted_date"`
	CollectedAt     time.Time  `json:"collected_at"`
	RequirementType string     `json:"requirement_type"`
}

type EvidenceJobs struct {
	Skill Skill         `json:"skill"`
	Total int           `json:"total"`
	Jobs  []EvidenceJob `json:"jobs"`
}

type GapItem struct {
	DemandItem
	PriorityScore float64 `json:"priority_score"`
}

type Gap struct {
	Role Role `json:"role"`
	MarketSample
	ThresholdPct float64 `json:"threshold_pct"`
	// CoveragePct: porsi total permintaan (skill di atas ambang) yang sudah dikuasai pengguna.
	CoveragePct float64      `json:"coverage_pct"`
	Gaps        []GapItem    `json:"gaps"`
	Owned       []DemandItem `json:"owned"`
}

var (
	ResourceTypes     = []string{"course", "docs", "video", "article", "book", "tutorial"}
	ResourceLanguages = []string{"id", "en"}
)

type LearningResource struct {
	ID       uuid.UUID `json:"id"`
	SkillID  int       `json:"skill_id"`
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	Type     string    `json:"type"`
	Level    *string   `json:"level"`
	Language string    `json:"language"`
	IsFree   bool      `json:"is_free"`
	EstHours *float64  `json:"est_hours"`
}

type ResourceInput struct {
	SkillID  int
	Title    string
	URL      string
	Type     string
	Level    *string
	Language string
	IsFree   bool
	EstHours *float64
}

type Roadmap struct {
	ID           uuid.UUID     `json:"id"`
	Version      int           `json:"version"`
	Role         Role          `json:"role"`
	ModelName    *string       `json:"model_name"`
	GeneratedAt  time.Time     `json:"generated_at"`
	HoursPerWeek *int16        `json:"hours_per_week"`
	TotalWeeks   float64       `json:"total_weeks"`
	Nodes        []RoadmapNode `json:"nodes"`
	// Edges: prerequisite -> skill, hanya di antara node roadmap (untuk React Flow).
	Edges []RoadmapEdge `json:"edges"`
}

type RoadmapNode struct {
	ID            uuid.UUID          `json:"id"`
	SkillID       int                `json:"skill_id"`
	Name          string             `json:"name"`
	Slug          string             `json:"slug"`
	Category      string             `json:"category"`
	OrderIndex    int                `json:"order_index"`
	Stage         string             `json:"stage"`
	PriorityScore *float64           `json:"priority_score"`
	DemandPct     *float64           `json:"demand_pct"`
	EstWeeks      *float64           `json:"est_weeks"`
	Rationale     *string            `json:"rationale"`
	Status        string             `json:"status"`
	Resources     []LearningResource `json:"resources"`
}

type RoadmapEdge struct {
	From int `json:"from_skill_id"`
	To   int `json:"to_skill_id"`
}

// RoadmapNodeInput: node yang akan disimpan (urutan final dari topological sort).
type RoadmapNodeInput struct {
	SkillID       int
	OrderIndex    int
	Stage         string
	PriorityScore *float64
	DemandPct     *float64
	EstWeeks      float64
	Rationale     string
}

// --- kontrak /internal/roadmap/explain (ai-service) ---

type ExplainRequest struct {
	Role    string         `json:"role"`
	Profile ExplainProfile `json:"profile"`
	Nodes   []ExplainNode  `json:"nodes"`
}

type ExplainProfile struct {
	Education    *string  `json:"education"`
	CurrentJob   *string  `json:"current_job"`
	HoursPerWeek int      `json:"hours_per_week"`
	Skills       []string `json:"skills"`
}

type ExplainNode struct {
	SkillID       int      `json:"skill_id"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Stage         string   `json:"stage"`
	DemandPct     *float64 `json:"demand_pct"`
	Prerequisites []string `json:"prerequisites"`
	// RequiredFor diisi untuk skill yang masuk roadmap karena prasyarat skill lain.
	RequiredFor []string `json:"required_for"`
}

type ExplainResponse struct {
	Model string          `json:"model"`
	Nodes []ExplainedNode `json:"nodes"`
}

type ExplainedNode struct {
	SkillID   int     `json:"skill_id"`
	Rationale string  `json:"rationale"`
	EstWeeks  float64 `json:"est_weeks"`
}
