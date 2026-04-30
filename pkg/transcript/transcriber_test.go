package transcript

import (
	"testing"
)

func TestNewVideoTranscriber_DefaultIsSupadata(t *testing.T) {
	t.Setenv("TRANSCRIPT_PROVIDER", "")
	t.Setenv("SUPADATA_API_KEY", "test-key")

	got := NewVideoTranscriber()
	if _, ok := got.(*SupadataTranscriber); !ok {
		t.Errorf("default provider = %T, want *SupadataTranscriber", got)
	}
}

func TestNewVideoTranscriber_ExplicitSupadata(t *testing.T) {
	t.Setenv("TRANSCRIPT_PROVIDER", "supadata")
	t.Setenv("SUPADATA_API_KEY", "test-key")

	got := NewVideoTranscriber()
	s, ok := got.(*SupadataTranscriber)
	if !ok {
		t.Fatalf("provider = %T, want *SupadataTranscriber", got)
	}
	if s.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", s.APIKey, "test-key")
	}
}

func TestNewVideoTranscriber_YouTube(t *testing.T) {
	t.Setenv("TRANSCRIPT_PROVIDER", "youtube")

	got := NewVideoTranscriber()
	if _, ok := got.(*YouTubeTranscriber); !ok {
		t.Errorf("provider = %T, want *YouTubeTranscriber", got)
	}
}

func TestNewVideoTranscriber_UnknownDefaultsToSupadata(t *testing.T) {
	t.Setenv("TRANSCRIPT_PROVIDER", "made-up-provider")
	t.Setenv("SUPADATA_API_KEY", "k")

	got := NewVideoTranscriber()
	if _, ok := got.(*SupadataTranscriber); !ok {
		t.Errorf("unknown value provider = %T, want *SupadataTranscriber fallback", got)
	}
}
