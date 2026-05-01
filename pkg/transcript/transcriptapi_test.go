package transcript

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestTranscriptAPI(t *testing.T, baseURL string) *TranscriptAPITranscriber {
	t.Helper()
	return &TranscriptAPITranscriber{
		APIKey:     "test-key",
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
		RetryDelay: time.Millisecond,
	}
}

func TestTranscriptAPI_Success(t *testing.T) {
	var gotAuth, gotPath, gotVideoURL, gotFormat, gotTimestamp string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotVideoURL = r.URL.Query().Get("video_url")
		gotFormat = r.URL.Query().Get("format")
		gotTimestamp = r.URL.Query().Get("include_timestamp")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"video_id":"vid123","language":"en","transcript":"hello world"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	tr := newTestTranscriptAPI(t, srv.URL)
	text, lang, err := tr.Transcribe(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
	if lang != "en" {
		t.Errorf("lang = %q, want %q", lang, "en")
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
	}
	if gotPath != "/youtube/transcript" {
		t.Errorf("path = %q, want %q", gotPath, "/youtube/transcript")
	}
	if gotVideoURL != "vid123" {
		t.Errorf("video_url = %q, want %q", gotVideoURL, "vid123")
	}
	if gotFormat != "text" {
		t.Errorf("format = %q, want %q", gotFormat, "text")
	}
	if gotTimestamp != "false" {
		t.Errorf("include_timestamp = %q, want %q", gotTimestamp, "false")
	}
}

func TestTranscriptAPI_MissingAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called when API key is missing")
	}))
	defer srv.Close()

	tr := &TranscriptAPITranscriber{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, _, err := tr.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Errorf("error = %v, want it to mention API key", err)
	}
}

func TestTranscriptAPI_EmptyVideoID(t *testing.T) {
	tr := newTestTranscriptAPI(t, "http://example.invalid")
	_, _, err := tr.Transcribe(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty videoID, got nil")
	}
}

func TestTranscriptAPI_NonRetryableErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"401 unauthorized", http.StatusUnauthorized},
		{"402 payment required", http.StatusPaymentRequired},
		{"404 not found", http.StatusNotFound},
		{"500 server error", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&count, 1)
				w.WriteHeader(tt.status)
				w.Write([]byte(`{"detail":"error"}`)) //nolint:errcheck
			}))
			defer srv.Close()

			tr := newTestTranscriptAPI(t, srv.URL)
			_, _, err := tr.Transcribe(context.Background(), "vid123")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := atomic.LoadInt32(&count); got != 1 {
				t.Errorf("requests = %d, want 1 (non-retryable)", got)
			}
		})
	}
}

func TestTranscriptAPI_408Retry(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n < 2 {
			w.WriteHeader(http.StatusRequestTimeout)
			w.Write([]byte(`{"detail":"timeout"}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"video_id":"vid123","language":"en","transcript":"retried text"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	tr := newTestTranscriptAPI(t, srv.URL)
	text, _, err := tr.Transcribe(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if text != "retried text" {
		t.Errorf("text = %q, want %q", text, "retried text")
	}
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}

func TestTranscriptAPI_429Retry(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n < 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"detail":"rate limited"}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"video_id":"vid123","language":"en","transcript":"rate limit text"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	tr := newTestTranscriptAPI(t, srv.URL)
	text, _, err := tr.Transcribe(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if text != "rate limit text" {
		t.Errorf("text = %q, want %q", text, "rate limit text")
	}
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}

func TestTranscriptAPI_503Retry(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"detail":"unavailable"}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"video_id":"vid123","language":"en","transcript":"recovered text"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	tr := newTestTranscriptAPI(t, srv.URL)
	text, _, err := tr.Transcribe(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if text != "recovered text" {
		t.Errorf("text = %q, want %q", text, "recovered text")
	}
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}

func TestTranscriptAPI_RetriesExhausted(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusRequestTimeout)
		w.Write([]byte(`{"detail":"timeout"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	tr := newTestTranscriptAPI(t, srv.URL)
	_, _, err := tr.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error after retries exhausted, got nil")
	}
	if got := atomic.LoadInt32(&count); got != maxTranscriptAPIRetries {
		t.Errorf("requests = %d, want %d", got, maxTranscriptAPIRetries)
	}
}

func TestTranscriptAPI_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`)) //nolint:errcheck
	}))
	defer srv.Close()

	tr := newTestTranscriptAPI(t, srv.URL)
	_, _, err := tr.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestTranscriptAPI_ContextCancelledDuringRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"detail":"unavailable"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	tr := &TranscriptAPITranscriber{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		HTTPClient: http.DefaultClient,
		RetryDelay: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	done := make(chan error, 1)
	go func() {
		_, _, err := tr.Transcribe(ctx, "vid123")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from cancelled context, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Transcribe did not return after context cancellation")
	}
}
