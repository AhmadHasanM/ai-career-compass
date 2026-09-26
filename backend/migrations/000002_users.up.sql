-- Sesi anonim tanpa login; id dikirim frontend lewat header X-Session-Id.
CREATE TABLE user_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_profiles (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id     UUID     NOT NULL UNIQUE REFERENCES user_sessions (id) ON DELETE CASCADE,
    education      TEXT,
    current_job    TEXT,
    target_role_id SMALLINT REFERENCES roles (id),
    hours_per_week SMALLINT CHECK (hours_per_week BETWEEN 1 AND 80),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_skills (
    profile_id  UUID NOT NULL REFERENCES user_profiles (id) ON DELETE CASCADE,
    skill_id    INT  NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    proficiency TEXT NOT NULL DEFAULT 'beginner'
        CHECK (proficiency IN ('beginner', 'intermediate', 'advanced')),
    source      TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'cv')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, skill_id)
);
