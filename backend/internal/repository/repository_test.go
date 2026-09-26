package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.RunWithDB(m)) }

func ptr[T any](v T) *T { return &v }

func TestSessionCreateAndTouch(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()
	repo := repository.NewSessionRepository(db)

	s, err := repo.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == uuid.Nil {
		t.Fatal("id sesi kosong")
	}

	ok, err := repo.Touch(ctx, s.ID)
	if err != nil || !ok {
		t.Fatalf("Touch sesi yang ada = %v, %v; ingin true", ok, err)
	}
	ok, err = repo.Touch(ctx, uuid.New())
	if err != nil || ok {
		t.Fatalf("Touch sesi acak = %v, %v; ingin false", ok, err)
	}
}

func TestSessionTouchUpdatesStaleLastActive(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()
	repo := repository.NewSessionRepository(db)
	s, _ := repo.Create(ctx)

	stale := time.Now().Add(-time.Hour)
	if _, err := db.Exec(ctx, `UPDATE user_sessions SET last_active_at = $2 WHERE id = $1`, s.ID, stale); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Touch(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	var last time.Time
	_ = db.QueryRow(ctx, `SELECT last_active_at FROM user_sessions WHERE id = $1`, s.ID).Scan(&last)
	if !last.After(stale.Add(time.Minute)) {
		t.Errorf("last_active_at tidak diperbarui: %v", last)
	}
}

func TestTaxonomyListSkills(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()
	repo := repository.NewTaxonomyRepository(db)

	all, err := repo.ListSkills(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("jumlah skill = %d, ingin 4", len(all))
	}

	llm, err := repo.ListSkills(ctx, "llm")
	if err != nil {
		t.Fatal(err)
	}
	if len(llm) != 1 || llm[0].Slug != "rag" {
		t.Fatalf("filter llm = %+v", llm)
	}
	if want := []int{testutil.SkillID(t, "python")}; len(llm[0].PrerequisiteIDs) != 1 || llm[0].PrerequisiteIDs[0] != want[0] {
		t.Errorf("prerequisite rag = %v, ingin %v", llm[0].PrerequisiteIDs, want)
	}

	data, _ := repo.ListSkills(ctx, "data")
	if got := data[0].Aliases; len(got) != 2 || got[0] != "postgres" || got[1] != "postgresql" {
		t.Errorf("alias postgresql = %v", got)
	}
}

func TestTaxonomyMissingSkillIDs(t *testing.T) {
	db := testutil.DB(t)
	repo := repository.NewTaxonomyRepository(db)
	py := testutil.SkillID(t, "python")

	missing, err := repo.MissingSkillIDs(context.Background(), []int{py, 99999, 88888})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 2 || missing[0] != 88888 || missing[1] != 99999 {
		t.Errorf("missing = %v, ingin [88888 99999]", missing)
	}
}

func TestProfileUpsertReplacesSkillsAndKeepsSource(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()
	sess, _ := repository.NewSessionRepository(db).Create(ctx)
	repo := repository.NewProfileRepository(db)

	if _, err := repo.GetBySession(ctx, sess.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("GetBySession sebelum profil dibuat = %v, ingin ErrNotFound", err)
	}

	py, rag, docker := testutil.SkillID(t, "python"), testutil.SkillID(t, "rag"), testutil.SkillID(t, "docker")
	role := testutil.RoleID(t, "ai-engineer")

	p, err := repo.Upsert(ctx, sess.ID, model.ProfileInput{
		Education:    ptr("S1 Informatika"),
		TargetRoleID: &role,
		HoursPerWeek: ptr[int16](10),
		Skills: []model.SkillInput{
			{SkillID: py, Proficiency: "intermediate"},
			{SkillID: docker, Proficiency: "beginner"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.TargetRole == nil || p.TargetRole.Slug != "ai-engineer" || len(p.Skills) != 2 {
		t.Fatalf("profil awal = %+v", p)
	}

	// Simulasikan skill Docker berasal dari ekstraksi CV (Fase 2).
	if _, err := db.Exec(ctx, `UPDATE user_skills SET source = 'cv' WHERE skill_id = $1`, docker); err != nil {
		t.Fatal(err)
	}

	p, err = repo.Upsert(ctx, sess.ID, model.ProfileInput{
		TargetRoleID: &role,
		HoursPerWeek: ptr[int16](5),
		Skills: []model.SkillInput{
			{SkillID: docker, Proficiency: "advanced"},
			{SkillID: rag, Proficiency: "beginner"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Education != nil {
		t.Errorf("education seharusnya dikosongkan oleh PUT, dapat %q", *p.Education)
	}
	got := map[string]model.UserSkill{}
	for _, s := range p.Skills {
		got[s.Slug] = s
	}
	if _, ok := got["python"]; ok {
		t.Error("python seharusnya terhapus")
	}
	if d := got["docker"]; d.Source != "cv" || d.Proficiency != "advanced" {
		t.Errorf("docker = %+v, ingin source cv + proficiency advanced", d)
	}
	if r := got["rag"]; r.Source != "manual" {
		t.Errorf("rag = %+v, ingin source manual", r)
	}

	// Daftar skill kosong menghapus semua skill.
	p, err = repo.Upsert(ctx, sess.ID, model.ProfileInput{TargetRoleID: &role, HoursPerWeek: ptr[int16](5), Skills: []model.SkillInput{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Skills) != 0 {
		t.Errorf("skills = %v, ingin kosong", p.Skills)
	}
}

func TestJobCreateAndDuplicate(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()
	repo := repository.NewJobRepository(db)

	in := model.JobInput{
		Title:       "AI Engineer",
		SourceName:  "Glints",
		SourceURL:   ptr("https://example.com/job/1"),
		CollectedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		RawText:     testutil.LongText(),
	}
	job, err := repo.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if job.ExtractionStatus != "pending" {
		t.Errorf("status = %s, ingin pending", job.ExtractionStatus)
	}

	_, err = repo.Create(ctx, in)
	var dup *repository.DuplicateError
	if !errors.As(err, &dup) || dup.ExistingID != job.ID.String() {
		t.Fatalf("insert kedua = %v, ingin DuplicateError dengan id %s", err, job.ID)
	}

	// Tanpa source_url tidak dianggap duplikat.
	in.SourceURL = nil
	if _, err := repo.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
}
