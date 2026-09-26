-- Per role dan skill: jumlah lowongan, demand_pct (% lowongan role yang menyebut skill),
-- dan required_pct (% yang mewajibkan). Hanya lowongan dengan ekstraksi selesai.
-- Di-refresh setiap ingestion selesai: REFRESH MATERIALIZED VIEW CONCURRENTLY skill_demand;
CREATE MATERIALIZED VIEW skill_demand AS
WITH role_totals AS (
    SELECT role_id, COUNT(*) AS total_jobs
    FROM job_postings
    WHERE extraction_status = 'done' AND role_id IS NOT NULL
    GROUP BY role_id
)
SELECT
    jp.role_id,
    js.skill_id,
    COUNT(*)                                                    AS job_count,
    COUNT(*) FILTER (WHERE js.requirement_type = 'required')    AS required_count,
    rt.total_jobs,
    ROUND(100.0 * COUNT(*) / rt.total_jobs, 2)                  AS demand_pct,
    ROUND(100.0 * COUNT(*) FILTER (WHERE js.requirement_type = 'required') / rt.total_jobs, 2) AS required_pct
FROM job_skills js
JOIN job_postings jp ON jp.id = js.job_id AND jp.extraction_status = 'done'
JOIN role_totals rt ON rt.role_id = jp.role_id
GROUP BY jp.role_id, js.skill_id, rt.total_jobs;

-- Unique index wajib agar REFRESH ... CONCURRENTLY bisa dipakai.
CREATE UNIQUE INDEX uq_skill_demand_role_skill ON skill_demand (role_id, skill_id);
CREATE INDEX idx_skill_demand_role_pct ON skill_demand (role_id, demand_pct DESC);
