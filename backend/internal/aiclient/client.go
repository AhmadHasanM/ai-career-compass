// Package aiclient memanggil endpoint /internal milik ai-service dengan header X-Internal-Token.
package aiclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// ProcessJob meminta ai-service memproses lowongan di background (ai-service membalas 202).
func (c *Client) ProcessJob(ctx context.Context, jobID uuid.UUID) error {
	url := fmt.Sprintf("%s/internal/jobs/%s/process", c.baseURL, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ai-service tidak terjangkau: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ai-service membalas %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
