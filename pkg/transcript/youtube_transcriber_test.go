package transcript

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYouTubeTranscriber_NoCaptions(t *testing.T) {
	orig := listCaptionsFn
	defer func() { listCaptionsFn = orig }()
	listCaptionsFn = func(string) ([]Caption, error) {
		return []Caption{}, nil
	}

	y := &YouTubeTranscriber{}
	_, _, err := y.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error for no captions, got nil")
	}
}

func TestYouTubeTranscriber_PicksFirstCaption(t *testing.T) {
	xmlBody := `<transcript><text>First track text</text></transcript>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(xmlBody)) //nolint:errcheck
	}))
	defer srv.Close()

	orig := listCaptionsFn
	defer func() { listCaptionsFn = orig }()
	listCaptionsFn = func(string) ([]Caption, error) {
		return []Caption{
			{LanguageCode: "en", BaseUrl: srv.URL},
			{LanguageCode: "es", BaseUrl: "http://should-not-be-used"},
		}, nil
	}

	y := &YouTubeTranscriber{}
	text, lang, err := y.Transcribe(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if lang != "en" {
		t.Errorf("lang = %q, want %q", lang, "en")
	}
	if text == "" || text != "First track text " {
		t.Errorf("text = %q, want %q", text, "First track text ")
	}
}

func TestYouTubeTranscriber_ListError(t *testing.T) {
	orig := listCaptionsFn
	defer func() { listCaptionsFn = orig }()
	listCaptionsFn = func(string) ([]Caption, error) {
		return nil, errors.New("network down")
	}

	y := &YouTubeTranscriber{}
	_, _, err := y.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestYouTubeTranscriber_ExtractError(t *testing.T) {
	orig := listCaptionsFn
	defer func() { listCaptionsFn = orig }()
	listCaptionsFn = func(string) ([]Caption, error) {
		return []Caption{{LanguageCode: "en", BaseUrl: "http://127.0.0.1:0"}}, nil
	}

	y := &YouTubeTranscriber{}
	_, _, err := y.Transcribe(context.Background(), "vid123")
	if err == nil {
		t.Fatal("expected error from unreachable caption URL, got nil")
	}
}
