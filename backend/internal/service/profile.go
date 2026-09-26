package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

const (
	maxTextField     = 200
	maxProfileSkills = 100
)

// ProfileRequest adalah body PUT /api/profile. PUT mengganti seluruh profil,
// termasuk daftar skill: skill yang tidak dikirim akan dihapus.
type ProfileRequest struct {
	Education    *string        `json:"education"`
	CurrentJob   *string        `json:"current_job"`
	TargetRoleID *int16         `json:"target_role_id"`
	HoursPerWeek *int16         `json:"hours_per_week"`
	Skills       []SkillRequest `json:"skills"`
}

type SkillRequest struct {
	SkillID     int    `json:"skill_id"`
	Proficiency string `json:"proficiency"`
}

type ProfileService struct {
	profiles *repository.ProfileRepository
	taxonomy *repository.TaxonomyRepository
}

func NewProfileService(profiles *repository.ProfileRepository, taxonomy *repository.TaxonomyRepository) *ProfileService {
	return &ProfileService{profiles: profiles, taxonomy: taxonomy}
}

func (s *ProfileService) Get(ctx context.Context, sessionID uuid.UUID) (*model.Profile, error) {
	p, err := s.profiles.GetBySession(ctx, sessionID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	return p, err
}

func (s *ProfileService) Save(ctx context.Context, sessionID uuid.UUID, req ProfileRequest) (*model.Profile, error) {
	in, err := s.validate(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.profiles.Upsert(ctx, sessionID, in)
}

func (s *ProfileService) validate(ctx context.Context, req ProfileRequest) (model.ProfileInput, error) {
	var v validator
	in := model.ProfileInput{
		Education:    cleanOptional(req.Education),
		CurrentJob:   cleanOptional(req.CurrentJob),
		TargetRoleID: req.TargetRoleID,
		HoursPerWeek: req.HoursPerWeek,
		Skills:       make([]model.SkillInput, 0, len(req.Skills)),
	}

	if tooLong(in.Education, maxTextField) {
		v.add("education", fmt.Sprintf("maksimal %d karakter", maxTextField))
	}
	if tooLong(in.CurrentJob, maxTextField) {
		v.add("current_job", fmt.Sprintf("maksimal %d karakter", maxTextField))
	}
	if in.HoursPerWeek == nil {
		v.add("hours_per_week", "wajib diisi")
	} else if *in.HoursPerWeek < 1 || *in.HoursPerWeek > 80 {
		v.add("hours_per_week", "harus antara 1 dan 80")
	}

	if in.TargetRoleID == nil {
		v.add("target_role_id", "wajib diisi")
	} else if ok, err := s.taxonomy.RoleExists(ctx, *in.TargetRoleID); err != nil {
		return in, err
	} else if !ok {
		v.add("target_role_id", "role tidak ditemukan")
	}

	if len(req.Skills) > maxProfileSkills {
		v.add("skills", fmt.Sprintf("maksimal %d skill", maxProfileSkills))
	}
	seen := map[int]bool{}
	ids := make([]int, 0, len(req.Skills))
	for i, sk := range req.Skills {
		field := fmt.Sprintf("skills[%d]", i)
		prof := strings.TrimSpace(sk.Proficiency)
		if prof == "" {
			prof = "beginner"
		}
		switch {
		case sk.SkillID <= 0:
			v.add(field+".skill_id", "wajib diisi")
		case seen[sk.SkillID]:
			v.add(field+".skill_id", "skill duplikat")
		}
		if !oneOf(&prof, model.Proficiencies) {
			v.add(field+".proficiency", "harus salah satu dari: "+strings.Join(model.Proficiencies, ", "))
		}
		seen[sk.SkillID] = true
		ids = append(ids, sk.SkillID)
		in.Skills = append(in.Skills, model.SkillInput{SkillID: sk.SkillID, Proficiency: prof})
	}

	if err := v.err(); err != nil {
		return in, err
	}

	missing, err := s.taxonomy.MissingSkillIDs(ctx, ids)
	if err != nil {
		return in, err
	}
	if len(missing) > 0 {
		v.add("skills", fmt.Sprintf("skill tidak ditemukan: %v", missing))
	}
	return in, v.err()
}
