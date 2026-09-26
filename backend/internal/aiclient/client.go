// Package aiclient memanggil endpoint /internal milik ai-service dengan header X-Internal-Token.
package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

// triggerTimeout: pemicu background job (ai-service langsung membalas 202).
const triggerTimeout = 10 * time.Second

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New membuat client tanpa timeout global; setiap panggilan memakai deadline dari context,
// karena stream chat bisa berlangsung jauh lebih lama daripada pemicu job.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{},
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) do(req *http.Request, wantStatus int) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai-service tidak terjangkau: %w", err)
	}
	if resp.StatusCode != wantStatus {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("ai-service membalas %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func (c *Client) trigger(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	req, err := c.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req, http.StatusAccepted)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// ProcessJob meminta ai-service memproses lowongan di background.
func (c *Client) ProcessJob(ctx context.Context, jobID uuid.UUID) error {
	return c.trigger(ctx, fmt.Sprintf("/internal/jobs/%s/process", jobID))
}

// EmbedResource meminta ai-service meng-embed sumber belajar di background.
func (c *Client) EmbedResource(ctx context.Context, resourceID uuid.UUID) error {
	return c.trigger(ctx, fmt.Sprintf("/internal/resources/%s/embed", resourceID))
}

// ExplainRoadmap meminta alasan dan estimasi durasi per node roadmap.
func (c *Client) ExplainRoadmap(ctx context.Context, in model.ExplainRequest) (*model.ExplainResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/internal/roadmap/explain", in)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out model.ExplainResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("respons explain tidak valid: %w", err)
	}
	return &out, nil
}
