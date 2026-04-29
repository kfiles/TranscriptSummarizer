//go:build integration

package transcript

import (
	"testing"
)

// TestIntegrationTranscriptExtract retrieves captions for a known public video and
// verifies that an English transcript can be extracted with non-empty text.
func TestIntegrationTranscriptExtract(t *testing.T) {
	const videoID = "zqYFtk5e8Pk"

	captions, err := ListVideoCaptions(videoID)
	if err != nil {
		t.Fatalf("ListVideoCaptions(%q): %v", videoID, err)
	}
	if len(captions) == 0 {
		t.Fatalf("ListVideoCaptions(%q): no caption tracks found", videoID)
	}

	var enCaption *Caption
	for i := range captions {
		if captions[i].LanguageCode == "en" {
			enCaption = &captions[i]
			break
		}
	}
	if enCaption == nil {
		t.Fatalf("no English caption track found for video %q; available: %v", videoID, captions)
	}

	text, err := enCaption.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText(): %v", err)
	}
	if text == "" {
		t.Errorf("ExtractText() returned empty transcript for video %q language %q", videoID, enCaption.LanguageCode)
	}

	if enCaption.LanguageCode != "en" {
		t.Errorf("language = %q, want %q", enCaption.LanguageCode, "en")
	}
}
