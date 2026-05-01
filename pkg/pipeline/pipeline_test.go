package pipeline

import (
	"context"
	"errors"
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
func (m *mockFacade) UpsertPlaylist(ctx context.Context, c *mongo.Client, p *transcript.Playlist) error {
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

// fakeTranscriber returns canned text/lang/err. It records the videoID it was called with.
type fakeTranscriber struct {
	text       string
	lang       string
	err        error
	calledWith string
}

func (f *fakeTranscriber) Transcribe(_ context.Context, videoID string) (string, string, error) {
	f.calledWith = videoID
	return f.text, f.lang, f.err
}

// useTranscriber installs ft as the transcriber returned by newTranscriber for
// the duration of a test, and restores the original on cleanup.
func useTranscriber(t *testing.T, ft *fakeTranscriber) {
	t.Helper()
	orig := newTranscriber
	t.Cleanup(func() { newTranscriber = orig })
	newTranscriber = func() transcript.VideoTranscriber { return ft }
}

// useListNames stubs the listNames seam to return the given names (and no
// error) for the duration of the test. Call this in any test that reaches the
// listNames call site (i.e. transcription succeeds) to avoid a nil-client panic.
func useListNames(t *testing.T, names []string) {
	t.Helper()
	orig := listNames
	t.Cleanup(func() { listNames = orig })
	listNames = func(_ context.Context, _ *mongo.Client) ([]string, error) { return names, nil }
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

// TestRun_TranscribeError covers the case where transcription fails.
func TestRun_TranscribeError(t *testing.T) {
	useTranscriber(t, &fakeTranscriber{err: errors.New("network error")})

	facade := &mockFacade{}
	err := Run(context.Background(), facade, nil, testVideo())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestRun_EmptyTranscript covers the case where the transcriber returns no text.
func TestRun_EmptyTranscript(t *testing.T) {
	useTranscriber(t, &fakeTranscriber{text: "", lang: "en"})

	facade := &mockFacade{}
	err := Run(context.Background(), facade, nil, testVideo())
	if err == nil {
		t.Fatal("expected error for empty transcript, got nil")
	}
}

// TestRun_InsertVideoError covers the case where inserting a new video fails after the pipeline succeeds.
func TestRun_InsertVideoError(t *testing.T) {
	useTranscriber(t, &fakeTranscriber{text: "some text", lang: "en"})
	useListNames(t, nil)

	origDS := doSummarize
	defer func() { doSummarize = origDS }()
	doSummarize = func(_ context.Context, _ string, _ []string) (string, error) {
		return "## Summary", nil
	}

	dir := t.TempDir()
	t.Setenv("HUGO_CONTENT_DIR", dir)

	facade := &mockFacade{
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
// new, the transcriber returns text, summarization runs, and a markdown file is written.
func TestRun_NewVideoNewTranscript(t *testing.T) {
	ft := &fakeTranscriber{text: "Hello world transcript", lang: "en"}
	useTranscriber(t, ft)
	useListNames(t, []string{"Alice Smith", "Bob Jones"})

	origDS := doSummarize
	defer func() { doSummarize = origDS }()
	var gotNames []string
	doSummarize = func(_ context.Context, _ string, names []string) (string, error) {
		gotNames = names
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
	if ft.calledWith != "vid123" {
		t.Errorf("transcriber called with %q, want %q", ft.calledWith, "vid123")
	}
	if len(gotNames) == 0 {
		t.Errorf("doSummarize received no names; expected names from listNames stub")
	}

	mdPath := path.Join(dir, "2024", "March", "vid123.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("expected markdown file at %s: %v", mdPath, err)
	}
}

// TestRun_ExistingTranscript covers the case where the transcript already exists
// in the DB — summarization is skipped and the existing summary is used.
func TestRun_ExistingTranscript(t *testing.T) {
	useTranscriber(t, &fakeTranscriber{text: "Hello world transcript", lang: "en"})
	useListNames(t, nil)

	origDS := doSummarize
	defer func() { doSummarize = origDS }()
	summarizeCalled := false
	doSummarize = func(_ context.Context, _ string, _ []string) (string, error) {
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

// TestRun_TranscriberSelection verifies that Run uses whatever newTranscriber returns.
func TestRun_TranscriberSelection(t *testing.T) {
	ft := &fakeTranscriber{text: "x", lang: "en"}
	useTranscriber(t, ft)
	useListNames(t, nil)

	origDS := doSummarize
	defer func() { doSummarize = origDS }()
	doSummarize = func(_ context.Context, _ string, _ []string) (string, error) { return "summary", nil }

	dir := t.TempDir()
	t.Setenv("HUGO_CONTENT_DIR", dir)

	facade := &mockFacade{
		getVideoFn: func(_ context.Context, _ *mongo.Client, _ string) (*transcript.Video, error) {
			return testVideo(), nil
		},
	}

	if err := Run(context.Background(), facade, nil, testVideo()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if ft.calledWith == "" {
		t.Error("expected fakeTranscriber to be called via newTranscriber seam")
	}
}

// TestRun_TranscriptUnavailable_NewVideo verifies that when the transcriber signals
// ErrTranscriptUnavailable for a video not yet in the DB, Run inserts the video with
// Description "Transcript unavailable", skips summarization/Facebook, and returns nil.
func TestRun_TranscriptUnavailable_NewVideo(t *testing.T) {
	useTranscriber(t, &fakeTranscriber{err: transcript.ErrTranscriptUnavailable})

	summarizeCalled := false
	origDS := doSummarize
	defer func() { doSummarize = origDS }()
	doSummarize = func(_ context.Context, _ string, _ []string) (string, error) {
		summarizeCalled = true
		return "should not be called", nil
	}

	var insertedVideo *transcript.Video
	facade := &mockFacade{
		getVideoFn: func(_ context.Context, _ *mongo.Client, _ string) (*transcript.Video, error) {
			return nil, errors.New("not found")
		},
		insertVideoFn: func(_ context.Context, _ *mongo.Client, v *transcript.Video) error {
			insertedVideo = v
			return nil
		},
	}

	if err := Run(context.Background(), facade, nil, testVideo()); err != nil {
		t.Fatalf("Run returned error, want nil: %v", err)
	}
	if summarizeCalled {
		t.Error("doSummarize should not be called when transcript is unavailable")
	}
	if insertedVideo == nil {
		t.Fatal("InsertVideo should have been called for new video")
	}
	if insertedVideo.Description != "Transcript unavailable" {
		t.Errorf("Description = %q, want %q", insertedVideo.Description, "Transcript unavailable")
	}
}

// TestRun_TranscriptUnavailable_ExistingVideo verifies that when the transcriber signals
// ErrTranscriptUnavailable for a video already in the DB, Run does not call InsertVideo
// and returns nil.
func TestRun_TranscriptUnavailable_ExistingVideo(t *testing.T) {
	useTranscriber(t, &fakeTranscriber{err: transcript.ErrTranscriptUnavailable})

	insertCalled := false
	facade := &mockFacade{
		getVideoFn: func(_ context.Context, _ *mongo.Client, _ string) (*transcript.Video, error) {
			return testVideo(), nil // already exists
		},
		insertVideoFn: func(_ context.Context, _ *mongo.Client, _ *transcript.Video) error {
			insertCalled = true
			return nil
		},
	}

	if err := Run(context.Background(), facade, nil, testVideo()); err != nil {
		t.Fatalf("Run returned error, want nil: %v", err)
	}
	if insertCalled {
		t.Error("InsertVideo should not be called when video already exists")
	}
}
