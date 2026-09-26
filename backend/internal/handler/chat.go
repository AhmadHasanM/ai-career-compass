package handler

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/httpx"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/middleware"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/service"
)

// chatTimeout: batas total satu jawaban (retrieval + stream LLM).
const chatTimeout = 90 * time.Second

type ChatHandler struct {
	svc *service.ChatService
	log *slog.Logger
}

type chatRequest struct {
	Message string `json:"message"`
}

// Chat: POST /api/chat. Meneruskan SSE dari ai-service per event (flush tiap event) dan menyimpan
// pertanyaan + jawaban saat event `done` diterima.
func (h *ChatHandler) Chat(c *gin.Context) {
	var req chatRequest
	if !decodeJSON(c, &req) {
		return
	}
	sid := middleware.SessionID(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), chatTimeout)
	defer cancel()

	body, question, err := h.svc.Open(ctx, sid, req.Message)
	if errors.Is(err, service.ErrAIUnavailable) {
		_ = c.Error(err)
		httpx.AbortError(c, http.StatusServiceUnavailable, "ai_unavailable", "layanan AI sedang tidak tersedia, coba lagi nanti")
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	defer body.Close()

	h2 := c.Writer.Header()
	h2.Set("Content-Type", "text/event-stream")
	h2.Set("Cache-Control", "no-cache")
	h2.Set("Connection", "keep-alive")
	h2.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	err = forwardSSE(body, c.Writer, func(event string, data []byte) {
		if event != "done" {
			return
		}
		// Jawaban sudah lengkap: simpan walau klien memutus koneksi sesudahnya.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := h.svc.SaveDone(saveCtx, sid, question, data); err != nil {
			h.log.Error("gagal menyimpan chat", "session_id", sid, "error", err)
		}
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		h.log.Warn("stream chat terputus", "session_id", sid, "error", err)
	}
}

// forwardSSE menyalin stream SSE blok demi blok (dipisah baris kosong) ke w dan flush setiap blok,
// lalu memanggil onEvent dengan nama event dan data-nya.
func forwardSSE(r io.Reader, w gin.ResponseWriter, onEvent func(event string, data []byte)) error {
	reader := bufio.NewReaderSize(r, 64<<10)
	var (
		block bytes.Buffer
		event string
		data  []byte
	)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			block.Write(line)
			trimmed := bytes.TrimRight(line, "\r\n")
			switch {
			case len(trimmed) == 0:
				if block.Len() > len(line) {
					if _, werr := w.Write(block.Bytes()); werr != nil {
						return werr
					}
					w.Flush()
					if event != "" {
						onEvent(event, data)
					}
				}
				block.Reset()
				event, data = "", nil
			case bytes.HasPrefix(trimmed, []byte("event:")):
				event = string(bytes.TrimSpace(trimmed[len("event:"):]))
			case bytes.HasPrefix(trimmed, []byte("data:")):
				if len(data) > 0 {
					data = append(data, '\n') // beberapa baris data digabung dengan newline (spesifikasi SSE)
				}
				data = append(data, bytes.TrimSpace(trimmed[len("data:"):])...)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// History: GET /api/chat/history?limit=
func (h *ChatHandler) History(c *gin.Context) {
	limit, ok := queryInt(c, "limit")
	if !ok {
		return
	}
	msgs, err := h.svc.History(c.Request.Context(), middleware.SessionID(c), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}
