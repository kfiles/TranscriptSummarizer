package transcript

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultTranscriptAPIBaseURL = "https://transcriptapi.com/api/v2"
	defaultRetryDelay           = 2 * time.Second
	maxTranscriptAPIRetries     = 3
)

// TranscriptAPITranscriber implements VideoTranscriber by calling the TranscriptAPI.com API.
type TranscriptAPITranscriber struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	RetryDelay time.Duration // delay between retries for 408/429/503; 0 uses defaultRetryDelay
}

// NewTranscriptAPITranscriber returns a TranscriptAPITranscriber configured with sane defaults.
func NewTranscriptAPITranscriber(apiKey string) *TranscriptAPITranscriber {
	return &TranscriptAPITranscriber{
		APIKey:     apiKey,
		BaseURL:    defaultTranscriptAPIBaseURL,
		HTTPClient: http.DefaultClient,
		RetryDelay: defaultRetryDelay,
	}
}

type transcriptAPIResponse struct {
	VideoID    string `json:"video_id"`
	Language   string `json:"language"`
	Transcript string `json:"transcript"`
}

func (t *TranscriptAPITranscriber) Transcribe(ctx context.Context, videoID string) (string, string, error) {
	if t.APIKey == "" {
		return "", "", fmt.Errorf("transcriptapi: API key not configured (set TRANSCRIPTAPI_API_KEY)")
	}
	if videoID == "" {
		return "", "", fmt.Errorf("transcriptapi: empty videoID")
	}

	endpoint := fmt.Sprintf("%s/youtube/transcript?video_url=%s&format=text&include_timestamp=false",
		t.baseURL(), url.QueryEscape(videoID))

	var lastErr error
	for attempt := 0; attempt < maxTranscriptAPIRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", "", fmt.Errorf("transcriptapi: build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+t.APIKey)

		resp, err := t.httpClient().Do(req)
		if err != nil {
			return "", "", fmt.Errorf("transcriptapi: request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return "", "", fmt.Errorf("transcriptapi: read body: %w", readErr)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			var r transcriptAPIResponse
			if err := json.Unmarshal(body, &r); err != nil {
				return "", "", fmt.Errorf("transcriptapi: decode response: %w", err)
			}
			return r.Transcript, r.Language, nil

		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("transcriptapi: rate limited (429): %s", string(body))
			delay := t.retryDelay()
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, parseErr := strconv.Atoi(ra); parseErr == nil {
					delay = time.Duration(secs) * time.Second
				}
			}
			if attempt+1 < maxTranscriptAPIRetries {
				select {
				case <-ctx.Done():
					return "", "", ctx.Err()
				case <-time.After(delay):
				}
			}

		case http.StatusRequestTimeout, http.StatusServiceUnavailable:
			lastErr = fmt.Errorf("transcriptapi: status %d: %s", resp.StatusCode, string(body))
			if attempt+1 < maxTranscriptAPIRetries {
				select {
				case <-ctx.Done():
					return "", "", ctx.Err()
				case <-time.After(t.retryDelay()):
				}
			}

		case http.StatusNotFound:
			return "", "", fmt.Errorf("transcriptapi: %w", ErrTranscriptUnavailable)

		default:
			return "", "", fmt.Errorf("transcriptapi: unexpected status %d: %s", resp.StatusCode, string(body))
		}
	}
	return "", "", lastErr
}

func (t *TranscriptAPITranscriber) baseURL() string {
	if t.BaseURL == "" {
		return defaultTranscriptAPIBaseURL
	}
	return t.BaseURL
}

func (t *TranscriptAPITranscriber) httpClient() *http.Client {
	if t.HTTPClient == nil {
		return http.DefaultClient
	}
	return t.HTTPClient
}

func (t *TranscriptAPITranscriber) retryDelay() time.Duration {
	if t.RetryDelay == 0 {
		return defaultRetryDelay
	}
	return t.RetryDelay
}
