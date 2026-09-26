CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE roles (
    id         SMALLINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skills (
    id          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    category    TEXT        NOT NULL
        CHECK (category IN ('language', 'framework', 'llm', 'ml', 'mlops', 'cloud', 'data')),
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skill_aliases (
    id       INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    skill_id INT  NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    alias    TEXT NOT NULL UNIQUE CHECK (alias = lower(alias))
);
CREATE INDEX idx_skill_aliases_skill ON skill_aliases (skill_id);

-- DAG prerequisite untuk urutan roadmap (siklus dicegah di seeder / service).
CREATE TABLE skill_prerequisites (
    skill_id              INT NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    prerequisite_skill_id INT NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    PRIMARY KEY (skill_id, prerequisite_skill_id),
    CHECK (skill_id <> prerequisite_skill_id)
);
CREATE INDEX idx_skill_prereq_prereq ON skill_prerequisites (prerequisite_skill_id);
