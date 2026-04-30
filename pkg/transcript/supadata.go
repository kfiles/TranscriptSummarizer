package transcript

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultSupadataBaseURL = "https://api.supadata.ai/v1"
	defaultPollInterval    = time.Second
)

// SupadataTranscriber implements VideoTranscriber by calling the Supadata API.
//
// API reference: https://docs.supadata.ai
type SupadataTranscriber struct {
	APIKey       string
	BaseURL      string
	HTTPClient   *http.Client
	PollInterval time.Duration
}

// NewSupadataTranscriber returns a SupadataTranscriber configured with sane
// defaults. Override fields directly for tests or custom HTTP behaviour.
func NewSupadataTranscriber(apiKey string) *SupadataTranscriber {
	return &SupadataTranscriber{
		APIKey:       apiKey,
		BaseURL:      defaultSupadataBaseURL,
		HTTPClient:   http.DefaultClient,
		PollInterval: defaultPollInterval,
	}
}

type supadataTextResponse struct {
	Content        string   `json:"content"`
	Lang           string   `json:"lang"`
	AvailableLangs []string `json:"availableLangs"`
}

type supadataJobResponse struct {
	JobID string `json:"jobId"`
}

type supadataJobStatusResponse struct {
	Status         string   `json:"status"`
	Content        string   `json:"content"`
	Lang           string   `json:"lang"`
	AvailableLangs []string `json:"availableLangs"`
	Error          string   `json:"error"`
}

func (s *SupadataTranscriber) Transcribe(ctx context.Context, videoID string) (string, string, error) {
	if s.APIKey == "" {
		return "", "", fmt.Errorf("supadata: API key not configured (set SUPADATA_API_KEY)")
	}
	if videoID == "" {
		return "", "", fmt.Errorf("supadata: empty videoID")
	}

	videoURL := "https://www.youtube.com/watch?v=" + videoID
	endpoint := fmt.Sprintf("%s/transcript?url=%s&text=true&mode=auto",
		s.baseURL(), url.QueryEscape(videoURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", fmt.Errorf("supadata: build request: %w", err)
	}
	req.Header.Set("x-api-key", s.APIKey)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", "", fmt.Errorf("supadata: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("supadata: read body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var r supadataTextResponse
		if err := json.Unmarshal(body, &r); err != nil {
			return "", "", fmt.Errorf("supadata: decode response: %w", err)
		}
		return r.Content, r.Lang, nil
	case http.StatusAccepted:
		var job supadataJobResponse
		if err := json.Unmarshal(body, &job); err != nil {
			return "", "", fmt.Errorf("supadata: decode job response: %w", err)
		}
		if job.JobID == "" {
			return "", "", fmt.Errorf("supadata: 202 response missing jobId")
		}
		return s.pollJob(ctx, job.JobID)
	default:
		return "", "", fmt.Errorf("supadata: unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func (s *SupadataTranscriber) pollJob(ctx context.Context, jobID string) (string, string, error) {
	endpoint := fmt.Sprintf("%s/transcript/%s", s.baseURL(), url.PathEscape(jobID))
	interval := s.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", "", fmt.Errorf("supadata: build poll request: %w", err)
		}
		req.Header.Set("x-api-key", s.APIKey)

		resp, err := s.httpClient().Do(req)
		if err != nil {
			return "", "", fmt.Errorf("supadata: poll request: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", "", fmt.Errorf("supadata: read poll body: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return "", "", fmt.Errorf("supadata: poll status %d: %s", resp.StatusCode, string(body))
		}

		var status supadataJobStatusResponse
		if err := json.Unmarshal(body, &status); err != nil {
			return "", "", fmt.Errorf("supadata: decode poll response: %w", err)
		}

		switch status.Status {
		case "completed":
			return status.Content, status.Lang, nil
		case "failed":
			return "", "", fmt.Errorf("supadata: job %s failed: %s", jobID, status.Error)
		case "queued", "active":
			// keep polling
		default:
			return "", "", fmt.Errorf("supadata: unknown job status %q", status.Status)
		}

		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *SupadataTranscriber) baseURL() string {
	if s.BaseURL == "" {
		return defaultSupadataBaseURL
	}
	return s.BaseURL
}

func (s *SupadataTranscriber) httpClient() *http.Client {
	if s.HTTPClient == nil {
		return http.DefaultClient
	}
	return s.HTTPClient
}
