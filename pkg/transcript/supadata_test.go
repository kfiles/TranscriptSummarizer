package transcript

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestSupadata(t *testing.T, baseURL string) *SupadataTranscriber {
	t.Helper()
	return &SupadataTranscriber{
		APIKey:       "test-key",
		BaseURL:      baseURL,
		HTTPClient:   http.DefaultClient,
		PollInterval: time.Millisecond,
	}
}

func TestSupadata_SyncSuccess(t *testing.T) {
	var gotAPIKey, gotPath, gotURL, gotText, gotMode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		gotPath = r.URL.Path
		gotURL = r.URL.Query().Get("url")
		gotText = r.URL.Query().Get("text")
		gotMode = r.URL.Query().Get("mode")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":"hello world","lang":"en","availableLangs":["en","es"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	text, lang, err := s.Transcribe(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
	if lang != "en" {
		t.Errorf("lang = %q, want %q", lang, "en")
	}
	if gotAPIKey != "test-key" {
		t.Errorf("x-api-key = %q, want %q", gotAPIKey, "test-key")
	}
	if gotPath != "/transcript" {
		t.Errorf("path = %q, want %q", gotPath, "/transcript")
	}
	if want := "https://www.youtube.com/watch?v=vid123"; gotURL != want {
		t.Errorf("url query = %q, want %q", gotURL, want)
	}
	if gotText != "true" {
		t.Errorf("text query = %q, want %q", gotText, "true")
	}
	if gotMode != "auto" {
		t.Errorf("mode query = %q, want %q", gotMode, "auto")
	}
}

func TestSupadata_AsyncJobCompleted(t *testing.T) {
	var pollCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/transcript"):
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"jobId":"job-1"}`)) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/transcript/job-1"):
			n := atomic.AddInt32(&pollCount, 1)
			if n < 2 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"active"}`)) //nolint:errcheck
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"completed","content":"async text","lang":"fr"}`)) //nolint:errcheck
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	text, lang, err := s.Transcribe(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if text != "async text" {
		t.Errorf("text = %q, want %q", text, "async text")
	}
	if lang != "fr" {
		t.Errorf("lang = %q, want %q", lang, "fr")
	}
	if got := atomic.LoadInt32(&pollCount); got < 2 {
		t.Errorf("pollCount = %d, want >= 2", got)
	}
}

func TestSupadata_AsyncJobFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/transcript"):
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"jobId":"job-2"}`)) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/transcript/job-2"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"failed","error":"transcription error"}`)) //nolint:errcheck
		}
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	_, _, err := s.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error from failed job, got nil")
	}
	if !strings.Contains(err.Error(), "transcription error") {
		t.Errorf("error = %v, want it to contain server error message", err)
	}
}

func TestSupadata_MissingAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called when API key is missing")
	}))
	defer srv.Close()

	s := &SupadataTranscriber{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, _, err := s.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Errorf("error = %v, want it to mention API key", err)
	}
}

func TestSupadata_404_ReturnsErrUnavailable(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	_, _, err := s.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !errors.Is(err, ErrTranscriptUnavailable) {
		t.Errorf("error = %v, want errors.Is(err, ErrTranscriptUnavailable) == true", err)
	}
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Errorf("requests = %d, want 1 (no retry on 404)", got)
	}
}

func TestSupadata_HTTPError(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"500 server error", http.StatusInternalServerError},
		{"403 forbidden", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(`{"error":"oops"}`)) //nolint:errcheck
			}))
			defer srv.Close()

			s := newTestSupadata(t, srv.URL)
			_, _, err := s.Transcribe(context.Background(), "vid123")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestSupadata_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`)) //nolint:errcheck
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	_, _, err := s.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestSupadata_ContextCancellationDuringPolling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/transcript") {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"jobId":"job-3"}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"active"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	s.PollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	done := make(chan error, 1)
	go func() {
		_, _, err := s.Transcribe(ctx, "vid123")
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

func TestSupadata_EmptyVideoID(t *testing.T) {
	s := newTestSupadata(t, "http://example.invalid")
	_, _, err := s.Transcribe(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty videoID, got nil")
	}
}

func TestSupadata_AcceptedMissingJobID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{}`)) //nolint:errcheck
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	_, _, err := s.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error for 202 with no jobId, got nil")
	}
}

func TestSupadata_UnknownJobStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/transcript") {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"jobId":"job-4"}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"banana"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	s := newTestSupadata(t, srv.URL)
	_, _, err := s.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error for unknown job status, got nil")
	}
}
