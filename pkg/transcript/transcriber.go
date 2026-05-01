package transcript

import (
	"context"
	"os"
)

// VideoTranscriber retrieves a transcript for a video by ID.
// Implementations may scrape YouTube directly or call a third-party API.
type VideoTranscriber interface {
	Transcribe(ctx context.Context, videoID string) (text string, languageCode string, err error)
}

// NewVideoTranscriber returns a VideoTranscriber selected by the
// TRANSCRIPT_PROVIDER env var. Recognized values:
//
//	"youtube"       - scrape YouTube caption tracks directly
//	"transcriptapi" - call the TranscriptAPI.com transcript API (TRANSCRIPTAPI_API_KEY)
//	"supadata"      - call the Supadata transcript API (default)
//
// Any other value (including empty) selects Supadata.
func NewVideoTranscriber() VideoTranscriber {
	switch os.Getenv("TRANSCRIPT_PROVIDER") {
	case "youtube":
		return &YouTubeTranscriber{}
	case "transcriptapi":
		return NewTranscriptAPITranscriber(os.Getenv("TRANSCRIPTAPI_API_KEY"))
	default:
		return NewSupadataTranscriber(os.Getenv("SUPADATA_API_KEY"))
	}
}
