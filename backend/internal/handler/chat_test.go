package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/config"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/testutil"
)

// fakeChatAI meniru /internal/chat: mencatat request, lalu mengirim event SSE yang diberikan.
type fakeChatAI struct {
	mu       sync.Mutex
	requests []model.ChatAIRequest
	events   string
	status   int
}

func (f *fakeChatAI) server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/chat" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var req model.ChatAIRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.requests = append(f.requests, req)
		f.mu.Unlock()
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// Kirim per blok dengan flush, seperti stream sungguhan.
		for _, block := range strings.SplitAfter(f.events, "\n\n") {
			_, _ = io.WriteString(w, block)
			w.(http.Flusher).Flush()
		}
	}))
}

const chunkID = "7b1d7a8e-0c8a-4a53-9a4f-1c1a2b3c4d5e"

var doneEvent = fmt.Sprintf(`{"answer":"Python paling dicari [1]. Data pasar [2].","citations":[`+
	`{"n":1,"source_type":"job_posting","chunk_id":"%s","title":"AI Engineer","url":"https://example.com/job"},`+
	`{"n":2,"source_type":"market_data","chunk_id":null,"title":"Statistik skill demand AI Engineer"}],`+
	`"model":"fake-llm","usage":{"prompt_tokens":300,"completion_tokens":40},"latency_ms":1234}`, chunkID)

var sseOK = "event: token\ndata: {\"text\":\"Python paling dicari \"}\n\n" +
	"event: token\ndata: {\"text\":\"[1]. Data pasar [2].\"}\n\n" +
	"event: done\ndata: " + doneEvent + "\n\n"

// chatCall mengirim POST /api/chat dan mengembalikan status + body mentah (SSE atau JSON).
func chatCall(t *testing.T, a *api, sid, message string) (int, string, http.Header) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"message": message})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Id", sid)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	return w.Code, w.Body.String(), w.Header()
}

func TestChatProxiesSSEAndSavesExchange(t *testing.T) {
	fake := &fakeChatAI{events: sseOK}
	ai := fake.server(t)
	defer ai.Close()
	a := newAPI(t, func(c *config.Config) { c.AIServiceURL, c.ChatPerMinute, c.RoadmapGeneratePerMinute = ai.URL, 100, 100 })
	db := testutil.DB(t) // DB() mengosongkan tabel: panggil sekali di awal
	seedJobs(t, db, marketJobs)
	h := userWithProfile(t, a)
	sid := h[1]

	code, body, header := chatCall(t, a, sid, "  Skill apa yang paling dicari?  ")
	if code != http.StatusOK || !strings.HasPrefix(header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("chat = %d %s", code, body)
	}
	if body != sseOK {
		t.Errorf("event tidak diteruskan apa adanya:\n%q\ningin\n%q", body, sseOK)
	}

	// Konteks yang dikirim ke ai-service
	req := fake.requests[0]
	if req.Question != "Skill apa yang paling dicari?" || req.User == nil || req.Market == nil {
		t.Fatalf("request ai = %+v", req)
	}
	if strings.Join(req.User.Skills, ",") != "Docker" || strings.Join(req.User.Gaps, ",") != "Retrieval-Augmented Generation,Python,PostgreSQL" {
		t.Errorf("user ctx = %+v", req.User)
	}
	if req.Market.TotalJobs != 4 || !req.Market.SmallSample || req.Market.Items[0].Name != "Python" {
		t.Errorf("market = %+v", req.Market)
	}

	// Riwayat tersimpan: pertanyaan + jawaban dengan sitasi lengkap
	r := a.do(http.MethodGet, "/api/chat/history", nil, h...)
	msgs, _ := r.Body["messages"].([]any)
	if r.Code != http.StatusOK || len(msgs) != 2 {
		t.Fatalf("history = %d %v", r.Code, r.Body)
	}
	q, ans := msgs[0].(map[string]any), msgs[1].(map[string]any)
	if q["role"] != "user" || q["content"] != "Skill apa yang paling dicari?" || ans["role"] != "assistant" {
		t.Errorf("urutan/isi riwayat = %v / %v", q, ans)
	}
	if cites, _ := ans["citations"].([]any); len(cites) != 2 {
		t.Errorf("citations = %v", ans["citations"])
	}
	var ids []string
	var tokens int
	if err := db.QueryRow(context.Background(), `
		SELECT cited_chunk_ids::text[], prompt_tokens FROM chat_messages WHERE role = 'assistant'`).Scan(&ids, &tokens); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != chunkID || tokens != 300 {
		t.Errorf("cited_chunk_ids = %v (sitasi data pasar tanpa chunk dilewati), prompt_tokens = %d", ids, tokens)
	}

	// Pertanyaan berikutnya membawa riwayat sebagai konteks
	if code, body, _ := chatCall(t, a, sid, "kalau untuk senior?"); code != http.StatusOK || len(fake.requests) != 2 {
		t.Fatalf("chat kedua = %d %s", code, body)
	}
	if hist := fake.requests[1].History; len(hist) != 2 || hist[0].Role != "user" || hist[1].Role != "assistant" {
		t.Errorf("history ke ai = %+v", hist)
	}
}

func TestChatWithoutProfileStillWorks(t *testing.T) {
	fake := &fakeChatAI{events: sseOK}
	ai := fake.server(t)
	defer ai.Close()
	a := newAPI(t, func(c *config.Config) { c.AIServiceURL, c.ChatPerMinute = ai.URL, 100 })
	seedJobs(t, testutil.DB(t), marketJobs)

	code, _, _ := chatCall(t, a, a.newSession(), "Halo")
	if code != http.StatusOK || fake.requests[0].User != nil || fake.requests[0].Market == nil {
		t.Fatalf("= %d, request = %+v", code, fake.requests[0])
	}
}

func TestChatErrorEventIsForwardedButNotSaved(t *testing.T) {
	fake := &fakeChatAI{events: "event: token\ndata: {\"text\":\"Py\"}\n\nevent: error\ndata: {\"message\":\"LLM timeout\"}\n\n"}
	ai := fake.server(t)
	defer ai.Close()
	a := newAPI(t, func(c *config.Config) { c.AIServiceURL, c.ChatPerMinute = ai.URL, 100 })
	sid := a.newSession()

	code, body, _ := chatCall(t, a, sid, "Halo")
	if code != http.StatusOK || !strings.Contains(body, "event: error") {
		t.Fatalf("= %d %s", code, body)
	}
	if r := a.do(http.MethodGet, "/api/chat/history", nil, "X-Session-Id", sid); len(r.Body["messages"].([]any)) != 0 {
		t.Errorf("jawaban gagal tidak boleh disimpan: %v", r.Body)
	}
}

func TestChatAIUnavailableAndValidation(t *testing.T) {
	fake := &fakeChatAI{status: http.StatusServiceUnavailable}
	ai := fake.server(t)
	defer ai.Close()
	a := newAPI(t, func(c *config.Config) { c.AIServiceURL, c.ChatPerMinute = ai.URL, 2 })
	sid := a.newSession()

	code, body, _ := chatCall(t, a, sid, "Halo")
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "ai_unavailable") {
		t.Errorf("ai-service 503 = %d %s", code, body)
	}
	if code, _, _ := chatCall(t, a, sid, "   "); code != http.StatusUnprocessableEntity {
		t.Errorf("pesan kosong = %d", code)
	}
	// limit 2 per menit per sesi sudah terpakai dua request di atas
	if code, _, _ := chatCall(t, a, sid, "Halo lagi"); code != http.StatusTooManyRequests {
		t.Errorf("rate limit per sesi = %d, ingin 429", code)
	}
	if code, _, _ := chatCall(t, a, a.newSession(), strings.Repeat("x", 2001)); code != http.StatusUnprocessableEntity {
		t.Errorf("pesan terlalu panjang = %d (sesi lain punya kuota sendiri)", code)
	}

	noAI := newAPI(t)
	if code, _, _ := chatCall(t, noAI, noAI.newSession(), "Halo"); code != http.StatusServiceUnavailable {
		t.Errorf("tanpa AI_SERVICE_URL = %d", code)
	}
}
