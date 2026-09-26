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
