-- Polymorphic (source_type + source_id): lowongan dan sumber belajar berbagi satu index hybrid search.
-- Dimensi 768 mengikuti intfloat/multilingual-e5-base; ganti lewat migrasi baru jika model embedding berubah.
CREATE TABLE document_chunks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type TEXT NOT NULL CHECK (source_type IN ('job_posting', 'learning_resource')),
    source_id   UUID NOT NULL,
    chunk_index INT  NOT NULL DEFAULT 0,
    section     TEXT,
    content     TEXT NOT NULL,
    embedding   vector(768),
    -- 'simple' karena teks campuran bahasa Indonesia dan Inggris.
    tsv         tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_type, source_id, chunk_index)
);
CREATE INDEX idx_document_chunks_embedding ON document_chunks USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_document_chunks_tsv ON document_chunks USING gin (tsv);
CREATE INDEX idx_document_chunks_metadata ON document_chunks USING gin (metadata jsonb_path_ops);
