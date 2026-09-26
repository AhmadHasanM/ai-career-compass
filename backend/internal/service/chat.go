package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

const (
	maxChatMessageChars = 2000
	chatHistoryTurns    = 6  // pesan terakhir yang dikirim sebagai konteks percakapan
	chatMarketItems     = 10 // statistik skill teratas sebagai sumber DATA PASAR
	chatGapItems        = 5
	maxHistoryLimit     = 200
)

// ErrAIUnavailable: ai-service tidak bisa dihubungi atau menolak sebelum stream dimulai.
var ErrAIUnavailable = errors.New("layanan AI tidak tersedia")

// ChatStreamer membuka stream SSE /internal/chat; pemanggil wajib menutup body.
type ChatStreamer interface {
	ChatStream(ctx context.Context, req model.ChatAIRequest) (io.ReadCloser, error)
}

type ChatService struct {
	gap      *GapService
	roadmaps *repository.RoadmapRepository
	chats    *repository.ChatRepository
	streamer ChatStreamer // nil = chat tidak tersedia
}

func NewChatService(gap *GapService, roadmaps *repository.RoadmapRepository, chats *repository.ChatRepository,
	streamer ChatStreamer) *ChatService {
	return &ChatService{gap: gap, roadmaps: roadmaps, chats: chats, streamer: streamer}
}

// Open memvalidasi pertanyaan, menyusun konteks (profil, gap, roadmap, data pasar, riwayat),
// lalu membuka stream ke ai-service.
func (s *ChatService) Open(ctx context.Context, sessionID uuid.UUID, message string) (io.ReadCloser, string, error) {
	question := strings.TrimSpace(message)
	switch n := utf8.RuneCountInString(question); {
	case n == 0:
		return nil, "", &ValidationError{Fields: map[string]string{"message": "wajib diisi"}}
	case n > maxChatMessageChars:
		return nil, "", &ValidationError{Fields: map[string]string{"message": fmt.Sprintf("maksimal %d karakter", maxChatMessageChars)}}
	}
	if s.streamer == nil {
		return nil, "", ErrAIUnavailable
	}

	req, err := s.buildRequest(ctx, sessionID, question)
	if err != nil {
		return nil, "", err
	}
	body, err := s.streamer.ChatStream(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrAIUnavailable, err)
	}
	return body, question, nil
}

func (s *ChatService) buildRequest(ctx context.Context, sessionID uuid.UUID, question string) (model.ChatAIRequest, error) {
	req := model.ChatAIRequest{Question: question, History: []model.ChatTurn{}}

	history, err := s.chats.Recent(ctx, sessionID, chatHistoryTurns)
	if err != nil {
		return req, err
	}
	for _, m := range history {
		req.History = append(req.History, model.ChatTurn{Role: m.Role, Content: m.Content})
	}

	// Tanpa profil chat tetap jalan: konteks pasar memakai role default.
	in, err := s.gap.load(ctx, sessionID)
	switch {
	case errors.Is(err, ErrProfileIncomplete):
		role, err := s.gap.insights.RoleBySlug(ctx, DefaultRoleSlug)
		if err != nil {
			return req, err
		}
		req.Market, err = s.market(ctx, role)
		return req, err
	case err != nil:
		return req, err
	}

	p := in.profile
	user := &model.ChatUser{
		Education: p.Education, CurrentJob: p.CurrentJob, TargetRole: &in.role.Name, HoursPerWeek: p.HoursPerWeek,
		Skills: []string{}, Gaps: []string{}, Roadmap: []string{},
	}
	for _, sk := range p.Skills {
		user.Skills = append(user.Skills, sk.Name)
	}
	gaps, _, _ := ComputeGap(in.demand, in.owned, s.gap.cfg)
	for i, g := range gaps {
		if i == chatGapItems {
			break
		}
		user.Gaps = append(user.Gaps, g.Name)
	}
	rm, err := s.roadmaps.Latest(ctx, p.ID, in.role.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return req, err
	}
	if rm != nil {
		for _, n := range rm.Nodes {
			entry := n.Name
			if n.Status == "done" {
				entry += " (selesai)"
			}
			user.Roadmap = append(user.Roadmap, entry)
		}
	}
	req.User = user
	req.Market = marketData(in.role, in.sample, in.demand)
	return req, nil
}

func (s *ChatService) market(ctx context.Context, role model.Role) (*model.ChatMarketData, error) {
	n, snapshot, err := s.gap.insights.Sample(ctx, role.ID, nil)
	if err != nil {
		return nil, err
	}
	demand, err := s.gap.insights.Demand(ctx, role.ID, nil, chatMarketItems)
	if err != nil {
		return nil, err
	}
	return marketData(role, model.MarketSample{TotalJobs: n, SnapshotDate: snapshot, SmallSample: n < smallSampleThreshold}, demand), nil
}

// marketData: nil jika belum ada lowongan, agar LLM tidak menyitir statistik kosong.
func marketData(role model.Role, sample model.MarketSample, demand []model.DemandItem) *model.ChatMarketData {
	if sample.TotalJobs == 0 || len(demand) == 0 {
		return nil
	}
	m := &model.ChatMarketData{Role: role.Name, TotalJobs: sample.TotalJobs, SmallSample: sample.SmallSample}
	if sample.SnapshotDate != nil {
		d := sample.SnapshotDate.Format("2006-01-02")
		m.SnapshotDate = &d
	}
	for i, d := range demand {
		if i == chatMarketItems {
			break
		}
		m.Items = append(m.Items, model.ChatMarketItem{Name: d.Name, DemandPct: d.DemandPct, RequiredPct: d.RequiredPct})
	}
	return m
}

// SaveDone menyimpan pertanyaan + jawaban dari data event `done`.
func (s *ChatService) SaveDone(ctx context.Context, sessionID uuid.UUID, question string, data []byte) error {
	var done model.ChatDone
	if err := json.Unmarshal(data, &done); err != nil {
		return fmt.Errorf("event done tidak valid: %w", err)
	}
	var cites []struct {
		ChunkID *string `json:"chunk_id"`
	}
	_ = json.Unmarshal(done.Citations, &cites)
	var ids []uuid.UUID
	for _, c := range cites {
		if c.ChunkID == nil {
			continue
		}
		if id, err := uuid.Parse(*c.ChunkID); err == nil {
			ids = append(ids, id)
		}
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return s.chats.SaveExchange(ctx, sessionID, question, done, ids)
}

func (s *ChatService) History(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > maxHistoryLimit {
		return nil, &ValidationError{Fields: map[string]string{"limit": "harus antara 1 dan 200"}}
	}
	msgs, err := s.chats.Recent(ctx, sessionID, limit)
	if msgs == nil {
		msgs = []model.ChatMessage{}
	}
	return msgs, err
}
