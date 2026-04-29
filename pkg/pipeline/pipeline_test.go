package pipeline

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// mockFacade implements db.Facade for tests. Only the methods exercised by
// pipeline are wired up; all others are no-ops that satisfy the interface.
type mockFacade struct {
	getVideoFn         func(ctx context.Context, client *mongo.Client, videoID string) (*transcript.Video, error)
	insertVideoFn      func(ctx context.Context, client *mongo.Client, v *transcript.Video) error
	getTranscriptFn    func(ctx context.Context, client *mongo.Client, videoID, lang string) (*transcript.Transcript, error)
	insertTranscriptFn func(ctx context.Context, client *mongo.Client, t *transcript.Transcript) error
	updateTranscriptFn func(ctx context.Context, client *mongo.Client, t *transcript.Transcript) error
}

func (m *mockFacade) GetVideo(ctx context.Context, c *mongo.Client, id string) (*transcript.Video, error) {
	if m.getVideoFn != nil {
		return m.getVideoFn(ctx, c, id)
	}
	return nil, errors.New("not found")
}
func (m *mockFacade) InsertVideo(ctx context.Context, c *mongo.Client, v *transcript.Video) error {
	if m.insertVideoFn != nil {
		return m.insertVideoFn(ctx, c, v)
	}
	return nil
}
func (m *mockFacade) GetTranscript(ctx context.Context, c *mongo.Client, videoID, lang string) (*transcript.Transcript, error) {
	if m.getTranscriptFn != nil {
		return m.getTranscriptFn(ctx, c, videoID, lang)
	}
	return nil, errors.New("not found")
}
func (m *mockFacade) InsertTranscript(ctx context.Context, c *mongo.Client, t *transcript.Transcript) error {
	if m.insertTranscriptFn != nil {
		return m.insertTranscriptFn(ctx, c, t)
	}
	return nil
}
func (m *mockFacade) UpdateTranscript(ctx context.Context, c *mongo.Client, t *transcript.Transcript) error {
	if m.updateTranscriptFn != nil {
		return m.updateTranscriptFn(ctx, c, t)
	}
	return nil
}

// no-op stubs for unused Facade methods
func (m *mockFacade) ListPlaylists(ctx context.Context, c *mongo.Client, channelID string) ([]*transcript.Playlist, error) {
	return nil, nil
}
func (m *mockFacade) GetPlaylist(ctx context.Context, c *mongo.Client, id string) (*transcript.Playlist, error) {
	return nil, nil
}
func (m *mockFacade) InsertPlaylist(ctx context.Context, c *mongo.Client, p *transcript.Playlist) error {
	return nil
}
func (m *mockFacade) UpdatePlaylist(ctx context.Context, c *mongo.Client, p *transcript.Playlist) error {
	return nil
}
func (m *mockFacade) DeletePlaylist(ctx context.Context, c *mongo.Client, id string) error { return nil }
func (m *mockFacade) ListVideos(ctx context.Context, c *mongo.Client, playlistID string) ([]*transcript.Video, error) {
	return nil, nil
}
func (m *mockFacade) UpdateVideo(ctx context.Context, c *mongo.Client, v *transcript.Video) error {
	return nil
}
func (m *mockFacade) DeleteVideo(ctx context.Context, c *mongo.Client, id string) error { return nil }
func (m *mockFacade) ListTranscripts(ctx context.Context, c *mongo.Client, videoID string) ([]*transcript.Transcript, error) {
	return nil, nil
}
func (m *mockFacade) DeleteTranscript(ctx context.Context, c *mongo.Client, id string) error {
	return nil
}

// captionServer starts a test HTTP server that returns a minimal YouTube caption XML.
func captionServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(body)) //nolint:errcheck
	}))
}

func testVideo() *transcript.Video {
	return &transcript.Video{
		VideoId:     "vid123",
		Title:       "Test Meeting",
		PublishedAt: time.Date(2024, time.March, 15, 10, 0, 0, 0, time.UTC),
	}
}

// TestWriteMarkdown verifies the markdown file is created with correct content and path.
func TestWriteMarkdown(t *testing.T) {
	dir := t.TempDir()
	v := testVideo()
	tr := &transcript.Transcript{
		VideoId:      v.VideoId,
		LanguageCode: "en",
		SummaryText:  "## Summary\n\nMeeting notes here.",
	}

	mdPath, err := writeMarkdown(v, tr, dir)
	if err != nil {
		t.Fatalf("writeMarkdown error: %v", err)
	}

	wantPath := path.Join(dir, "2024", "March", "vid123.md")
	if mdPath != wantPath {
		t.Errorf("mdPath = %q, want %q", mdPath, wantPath)
	}

	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	content := string(data)
	if want := "title = 'Test Meeting'"; !strings.Contains(content, want) {
		t.Errorf("markdown missing %q\ngot:\n%s", want, content)
	}
	if want := "## Summary"; !strings.Contains(content, want) {
		t.Errorf("markdown missing %q\ngot:\n%s", want, content)
	}

	// _index.md files should be created in year and month dirs
	for _, idx := range []string{
		path.Join(dir, "2024", indexName),
		path.Join(dir, "2024", "March", indexName),
	} {
		if _, err := os.Stat(idx); err != nil {
			t.Errorf("expected index file %s: %v", idx, err)
		}
	}
}

// TestRun_FetchCaptionsError covers the case where listing captions fails.
func TestRun_FetchCaptionsError(t *testing.T) {
	origLC := listCaptions
	defer func() { listCaptions = origLC }()
	listCaptions = func(string) ([]transcript.Caption, error) {
		return nil, errors.New("network error")
	}

	facade := &mockFacade{}
	err := Run(context.Background(), facade, nil, testVideo())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestRun_NoCaptions covers the case where a video has no caption tracks.
func TestRun_NoCaptions(t *testing.T) {
	origLC := listCaptions
	defer func() { listCaptions = origLC }()
	listCaptions = func(string) ([]transcript.Caption, error) {
		return []transcript.Caption{}, nil
	}

	facade := &mockFacade{}
	err := Run(context.Background(), facade, nil, testVideo())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestRun_InsertVideoError covers the case where inserting a new video fails.
func TestRun_InsertVideoError(t *testing.T) {
	origLC := listCaptions
	defer func() { listCaptions = origLC }()
	listCaptions = func(string) ([]transcript.Caption, error) {
		return []transcript.Caption{{LanguageCode: "en", BaseUrl: "http://unused"}}, nil
	}

	facade := &mockFacade{
		// GetVideo returns error → triggers InsertVideo
		getVideoFn: func(_ context.Context, _ *mongo.Client, _ string) (*transcript.Video, error) {
			return nil, errors.New("not found")
		},
		insertVideoFn: func(_ context.Context, _ *mongo.Client, _ *transcript.Video) error {
			return errors.New("db write error")
		},
	}

	err := Run(context.Background(), facade, nil, testVideo())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestRun_NewVideoNewTranscript is the happy-path: video and transcript are both
// new, captions are fetched, summarized, and a markdown file is written.
func TestRun_NewVideoNewTranscript(t *testing.T) {
	srv := captionServer(t, `<transcript><text>Hello world</text></transcript>`)
	defer srv.Close()

	origLC := listCaptions
	origDS := doSummarize
	defer func() {
		listCaptions = origLC
		doSummarize = origDS
	}()

	listCaptions = func(string) ([]transcript.Caption, error) {
		return []transcript.Caption{{LanguageCode: "en", BaseUrl: srv.URL}}, nil
	}
	doSummarize = func(_ context.Context, text string) (string, error) {
		return "## Summary\n\nTest summary.", nil
	}

	dir := t.TempDir()
	t.Setenv("HUGO_CONTENT_DIR", dir)

	facade := &mockFacade{
		getVideoFn: func(_ context.Context, _ *mongo.Client, _ string) (*transcript.Video, error) {
			return nil, errors.New("not found")
		},
	}

	if err := Run(context.Background(), facade, nil, testVideo()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mdPath := path.Join(dir, "2024", "March", "vid123.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("expected markdown file at %s: %v", mdPath, err)
	}
}

// TestRun_ExistingTranscript covers the case where the transcript already exists
// in the DB — summarization is skipped and the existing summary is used.
func TestRun_ExistingTranscript(t *testing.T) {
	srv := captionServer(t, `<transcript><text>Hello world</text></transcript>`)
	defer srv.Close()

	origLC := listCaptions
	origDS := doSummarize
	defer func() {
		listCaptions = origLC
		doSummarize = origDS
	}()

	listCaptions = func(string) ([]transcript.Caption, error) {
		return []transcript.Caption{{LanguageCode: "en", BaseUrl: srv.URL}}, nil
	}
	summarizeCalled := false
	doSummarize = func(_ context.Context, _ string) (string, error) {
		summarizeCalled = true
		return "should not be called", nil
	}

	existingTranscript := &transcript.Transcript{
		VideoId:      "vid123",
		LanguageCode: "en",
		SummaryText:  "## Cached Summary",
	}

	dir := t.TempDir()
	t.Setenv("HUGO_CONTENT_DIR", dir)

	facade := &mockFacade{
		getVideoFn: func(_ context.Context, _ *mongo.Client, _ string) (*transcript.Video, error) {
			return testVideo(), nil // video already exists
		},
		getTranscriptFn: func(_ context.Context, _ *mongo.Client, _, _ string) (*transcript.Transcript, error) {
			return existingTranscript, nil // transcript already exists
		},
	}

	if err := Run(context.Background(), facade, nil, testVideo()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if summarizeCalled {
		t.Error("doSummarize should not be called when transcript already exists")
	}

	data, err := os.ReadFile(path.Join(dir, "2024", "March", "vid123.md"))
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(data), "## Cached Summary") {
		t.Errorf("markdown should contain cached summary, got:\n%s", data)
	}
}
