CREATE TABLE job_postings (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title             TEXT     NOT NULL,
    company           TEXT,
    role_id           SMALLINT REFERENCES roles (id),
    level             TEXT CHECK (level IN ('intern', 'junior', 'mid', 'senior', 'lead')),
    location          TEXT,
    work_type         TEXT CHECK (work_type IN ('onsite', 'remote', 'hybrid')),
    source_name       TEXT     NOT NULL,
    source_url        TEXT,
    posted_date       DATE,
    collected_at      DATE     NOT NULL DEFAULT CURRENT_DATE,
    raw_text          TEXT     NOT NULL,
    extraction_status TEXT     NOT NULL DEFAULT 'pending'
        CHECK (extraction_status IN ('pending', 'done', 'failed', 'review')),
    extraction_error  TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_job_postings_source_url ON job_postings (source_url) WHERE source_url IS NOT NULL;
CREATE INDEX idx_job_postings_role_status ON job_postings (role_id, extraction_status);

CREATE TABLE job_skills (
    job_id           UUID NOT NULL REFERENCES job_postings (id) ON DELETE CASCADE,
    skill_id         INT  NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    requirement_type TEXT NOT NULL CHECK (requirement_type IN ('required', 'preferred')),
    PRIMARY KEY (job_id, skill_id)
);
CREATE INDEX idx_job_skills_skill ON job_skills (skill_id);

-- Antrean review skill hasil ekstraksi yang tidak cocok dengan taxonomy.
CREATE TABLE unmapped_skills (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID NOT NULL REFERENCES job_postings (id) ON DELETE CASCADE,
    raw_name        TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'mapped', 'ignored')),
    mapped_skill_id INT REFERENCES skills (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_unmapped_skills_status ON unmapped_skills (status);

CREATE TABLE learning_resources (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id   INT  NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    url        TEXT NOT NULL UNIQUE,
    type       TEXT NOT NULL CHECK (type IN ('course', 'docs', 'video', 'article', 'book', 'tutorial')),
    level      TEXT CHECK (level IN ('beginner', 'intermediate', 'advanced')),
    language   TEXT NOT NULL DEFAULT 'en' CHECK (language IN ('id', 'en')),
    is_free    BOOLEAN NOT NULL DEFAULT TRUE,
    est_hours  NUMERIC(5, 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_learning_resources_skill ON learning_resources (skill_id);
