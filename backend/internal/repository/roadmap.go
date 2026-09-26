package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type RoadmapRepository struct {
	db *pgxpool.Pool
}

func NewRoadmapRepository(db *pgxpool.Pool) *RoadmapRepository {
	return &RoadmapRepository{db: db}
}

// Prerequisites: skill_id -> daftar prerequisite_skill_id untuk seluruh taxonomy.
func (r *RoadmapRepository) Prerequisites(ctx context.Context) (map[int][]int, error) {
	rows, err := r.db.Query(ctx, `SELECT skill_id, prerequisite_skill_id FROM skill_prerequisites ORDER BY 1, 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int][]int{}
	for rows.Next() {
		var s, p int
		if err := rows.Scan(&s, &p); err != nil {
			return nil, err
		}
		out[s] = append(out[s], p)
	}
	return out, rows.Err()
}

type SkillInfo struct {
	ID       int
	Name     string
	Slug     string
	Category string
}

func (r *RoadmapRepository) SkillsByIDs(ctx context.Context, ids []int) (map[int]SkillInfo, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, slug, category FROM skills WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[SkillInfo])
	if err != nil {
		return nil, err
	}
	out := make(map[int]SkillInfo, len(list))
	for _, s := range list {
		out[s.ID] = s
	}
	return out, nil
}

// Save menyimpan roadmap versi berikutnya untuk (profil, role) beserta node-nodenya.
func (r *RoadmapRepository) Save(ctx context.Context, profileID uuid.UUID, roleID int16, modelName string, nodes []model.RoadmapNodeInput) (uuid.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	// Serialkan generate paralel untuk profil yang sama agar nomor versi tidak bentrok.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, profileID); err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO roadmaps (profile_id, role_id, version, model_name)
		SELECT $1, $2, COALESCE(max(version), 0) + 1, $3 FROM roadmaps WHERE profile_id = $1 AND role_id = $2
		RETURNING id`, profileID, roleID, modelName).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	for _, n := range nodes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO roadmap_nodes
				(roadmap_id, skill_id, order_index, stage, priority_score, demand_pct, est_weeks, rationale)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			id, n.SkillID, n.OrderIndex, n.Stage, n.PriorityScore, n.DemandPct, n.EstWeeks, n.Rationale); err != nil {
			return uuid.Nil, err
		}
	}
	return id, tx.Commit(ctx)
}

// Latest mengambil roadmap versi terbaru untuk (profil, role), lengkap dengan node dan edge.
func (r *RoadmapRepository) Latest(ctx context.Context, profileID uuid.UUID, roleID int16) (*model.Roadmap, error) {
	var rm model.Roadmap
	err := r.db.QueryRow(ctx, `
		SELECT rm.id, rm.version, rm.model_name, rm.generated_at, ro.id, ro.name, ro.slug
		FROM roadmaps rm JOIN roles ro ON ro.id = rm.role_id
		WHERE rm.profile_id = $1 AND rm.role_id = $2
		ORDER BY rm.version DESC LIMIT 1`, profileID, roleID).
		Scan(&rm.ID, &rm.Version, &rm.ModelName, &rm.GeneratedAt, &rm.Role.ID, &rm.Role.Name, &rm.Role.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT n.id, n.skill_id, s.name, s.slug, s.category, n.order_index, n.stage,
		       n.priority_score::float8, n.demand_pct::float8, n.est_weeks::float8, n.rationale, n.status
		FROM roadmap_nodes n JOIN skills s ON s.id = n.skill_id
		WHERE n.roadmap_id = $1 ORDER BY n.order_index`, rm.ID)
	if err != nil {
		return nil, err
	}
	rm.Nodes, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.RoadmapNode, error) {
		var n model.RoadmapNode
		err := row.Scan(&n.ID, &n.SkillID, &n.Name, &n.Slug, &n.Category, &n.OrderIndex, &n.Stage,
			&n.PriorityScore, &n.DemandPct, &n.EstWeeks, &n.Rationale, &n.Status)
		n.Resources = []model.LearningResource{}
		return n, err
	})
	if err != nil {
		return nil, err
	}

	rows, err = r.db.Query(ctx, `
		SELECT p.prerequisite_skill_id, p.skill_id
		FROM skill_prerequisites p
		JOIN roadmap_nodes a ON a.skill_id = p.skill_id AND a.roadmap_id = $1
		JOIN roadmap_nodes b ON b.skill_id = p.prerequisite_skill_id AND b.roadmap_id = $1
		ORDER BY 1, 2`, rm.ID)
	if err != nil {
		return nil, err
	}
	rm.Edges, err = pgx.CollectRows(rows, pgx.RowToStructByPos[model.RoadmapEdge])
	return &rm, err
}
