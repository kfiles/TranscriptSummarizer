package transcript

import (
	"context"
	"fmt"
)

// listCaptionsFn is overridden in tests; defaults to ListVideoCaptions.
var listCaptionsFn = ListVideoCaptions

// YouTubeTranscriber implements VideoTranscriber by scraping the public
// YouTube watch page and downloading the first caption track.
type YouTubeTranscriber struct{}

func (y *YouTubeTranscriber) Transcribe(ctx context.Context, videoID string) (string, string, error) {
	captions, err := listCaptionsFn(videoID)
	if err != nil {
		return "", "", fmt.Errorf("list captions: %w", err)
	}
	if len(captions) == 0 {
		return "", "", fmt.Errorf("no caption tracks found for video %s", videoID)
	}
	first := captions[0]
	text, err := first.ExtractText()
	if err != nil {
		return "", "", fmt.Errorf("extract caption text: %w", err)
	}
	return text, first.LanguageCode, nil
}
