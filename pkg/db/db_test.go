package db

import (
	"context"
	"testing"
	"time"

	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
)

// TestNewFacade verifies that NewFacade returns a non-nil Facade implementation.
func TestNewFacade(t *testing.T) {
	f := NewFacade()
	if f == nil {
		t.Fatal("NewFacade() returned nil")
	}
}

// The methods below are stubs that do not use the MongoDB client.
// Passing nil is safe for these specific implementations.

func TestListVideosReturnsEmpty(t *testing.T) {
	f := NewFacade()
	videos, err := f.ListVideos(context.Background(), nil, "pl1")
	if err != nil {
		t.Fatalf("ListVideos() unexpected error: %v", err)
	}
	if len(videos) != 0 {
		t.Errorf("ListVideos() len = %d, want 0", len(videos))
	}
}

func TestUpdateVideoReturnsNil(t *testing.T) {
	f := NewFacade()
	v := &transcript.Video{
		VideoId:     "v1",
		PlaylistId:  "pl1",
		Title:       "Test",
		PublishedAt: time.Now(),
	}
	err := f.UpdateVideo(context.Background(), nil, v)
	if err != nil {
		t.Errorf("UpdateVideo() unexpected error: %v", err)
	}
}

func TestDeleteVideoReturnsNil(t *testing.T) {
	f := NewFacade()
	err := f.DeleteVideo(context.Background(), nil, "v1")
	if err != nil {
		t.Errorf("DeleteVideo() unexpected error: %v", err)
	}
}

func TestListTranscriptsReturnsEmpty(t *testing.T) {
	f := NewFacade()
	transcripts, err := f.ListTranscripts(context.Background(), nil, "v1")
	if err != nil {
		t.Fatalf("ListTranscripts() unexpected error: %v", err)
	}
	if len(transcripts) != 0 {
		t.Errorf("ListTranscripts() len = %d, want 0", len(transcripts))
	}
}

func TestDeleteTranscriptReturnsNil(t *testing.T) {
	f := NewFacade()
	err := f.DeleteTranscript(context.Background(), nil, "v1")
	if err != nil {
		t.Errorf("DeleteTranscript() unexpected error: %v", err)
	}
}
