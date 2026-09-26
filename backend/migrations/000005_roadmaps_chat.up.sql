CREATE TABLE roadmaps (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id   UUID     NOT NULL REFERENCES user_profiles (id) ON DELETE CASCADE,
    role_id      SMALLINT NOT NULL REFERENCES roles (id),
    version      INT      NOT NULL DEFAULT 1,
    model_name   TEXT,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (profile_id, role_id, version)
);

CREATE TABLE roadmap_nodes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id     UUID NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    skill_id       INT  NOT NULL REFERENCES skills (id),
    order_index    INT  NOT NULL,
    stage          TEXT NOT NULL,
    priority_score NUMERIC(6, 3),
    demand_pct     NUMERIC(5, 2),
    est_weeks      NUMERIC(4, 1),
    rationale      TEXT,
    status         TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
    UNIQUE (roadmap_id, skill_id)
);
CREATE INDEX idx_roadmap_nodes_roadmap ON roadmap_nodes (roadmap_id, order_index);

CREATE TABLE chat_messages (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id        UUID NOT NULL REFERENCES user_sessions (id) ON DELETE CASCADE,
    role              TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content           TEXT NOT NULL,
    cited_chunk_ids   UUID[] NOT NULL DEFAULT '{}',
    latency_ms        INT,
    prompt_tokens     INT,
    completion_tokens INT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_chat_messages_session ON chat_messages (session_id, created_at);
