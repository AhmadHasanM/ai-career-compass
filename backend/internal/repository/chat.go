package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type ChatRepository struct {
	db *pgxpool.Pool
}

func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

// Recent mengembalikan n pesan terakhir sesi, urut lama -> baru.
func (r *ChatRepository) Recent(ctx context.Context, sessionID uuid.UUID, n int) ([]model.ChatMessage, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, role, content, citations, created_at FROM (
			SELECT * FROM chat_messages WHERE session_id = $1 ORDER BY created_at DESC LIMIT $2
		) recent ORDER BY created_at`, sessionID, n)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[model.ChatMessage])
}

// SaveExchange menyimpan pertanyaan dan jawaban dalam satu transaksi. created_at jawaban dibuat
// sedikit setelah pertanyaan agar urutan riwayat stabil.
func (r *ChatRepository) SaveExchange(ctx context.Context, sessionID uuid.UUID, question string, done model.ChatDone,
	citedChunkIDs []uuid.UUID) error {
	citations := done.Citations
	if len(citations) == 0 || string(citations) == "null" {
		citations = json.RawMessage("[]")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO chat_messages (session_id, role, content, created_at) VALUES ($1, 'user', $2, clock_timestamp())`,
		sessionID, question); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO chat_messages
			(session_id, role, content, cited_chunk_ids, citations, latency_ms, prompt_tokens, completion_tokens, created_at)
		VALUES ($1, 'assistant', $2, $3, $4, $5, $6, $7, clock_timestamp())`,
		sessionID, done.Answer, citedChunkIDs, citations, done.LatencyMs,
		done.Usage.PromptTokens, done.Usage.CompletionTokens); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
