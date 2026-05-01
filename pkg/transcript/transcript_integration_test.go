//go:build integration

package transcript

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegrationSupadataTranscribe calls the live Supadata API for a known
// public YouTube video and verifies a non-empty English transcript is returned.
// Skipped unless SUPADATA_API_KEY is set.
func TestIntegrationSupadataTranscribe(t *testing.T) {
	apiKey := os.Getenv("SUPADATA_API_KEY")
	if apiKey == "" {
		t.Skip("set SUPADATA_API_KEY to run Supadata integration tests")
	}

	const videoID = "zqYFtk5e8Pk"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := NewSupadataTranscriber(apiKey)
	text, lang, err := s.Transcribe(ctx, videoID)
	if err != nil {
		t.Fatalf("Transcribe(%q): %v", videoID, err)
	}
	if text == "" {
		t.Errorf("Transcribe(%q) returned empty transcript (lang=%q)", videoID, lang)
	}
	if lang != "en" {
		t.Errorf("lang = %q, want %q", lang, "en")
	}
}

// TestIntegrationTranscriptAPITranscribe calls the live TranscriptAPI.com API for a known
// public YouTube video and verifies a non-empty English transcript is returned.
// Skipped unless TRANSCRIPTAPI_API_KEY is set.
func TestIntegrationTranscriptAPITranscribe(t *testing.T) {
	apiKey := os.Getenv("TRANSCRIPTAPI_API_KEY")
	if apiKey == "" {
		t.Skip("set TRANSCRIPTAPI_API_KEY to run TranscriptAPI integration tests")
	}

	const videoID = "zqYFtk5e8Pk"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := NewTranscriptAPITranscriber(apiKey)
	text, lang, err := s.Transcribe(ctx, videoID)
	if err != nil {
		t.Fatalf("Transcribe(%q): %v", videoID, err)
	}
	if text == "" {
		t.Errorf("Transcribe(%q) returned empty transcript (lang=%q)", videoID, lang)
	}
	if lang != "en" {
		t.Errorf("lang = %q, want %q", lang, "en")
	}
}
