package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kfiles/transcriptsummarizer/pkg/db"
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// validFeedXML builds a minimal YouTube PubSubHubbub Atom payload.
func validFeedXML(videoID, channelID string) string {
	return `<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015">` +
		`<entry>` +
		`<yt:videoId>` + videoID + `</yt:videoId>` +
		`<yt:channelId>` + channelID + `</yt:channelId>` +
		`<title>Test Video</title>` +
		`</entry>` +
		`</feed>`
}

// makeEntry builds a PlaylistEntry for use in tests.
func makeEntry(videoID, pageToken string, position int64, publishedAt time.Time) *transcript.PlaylistEntry {
	return &transcript.PlaylistEntry{
		Video: &transcript.Video{
			VideoId:     videoID,
			Position:    position,
			PublishedAt: publishedAt,
		},
		PageToken: pageToken,
	}
}

// --- fakeFacade ---

type fakeFacade struct {
	playlists []*transcript.Playlist
	listErr   error
	videos    map[string]*transcript.Video // nil value means "not found"
	updated   []*transcript.Playlist
}

func newFake(playlists []*transcript.Playlist) *fakeFacade {
	return &fakeFacade{
		playlists: playlists,
		videos:    map[string]*transcript.Video{},
	}
}

func (f *fakeFacade) ListPlaylists(_ context.Context, _ *mongo.Client, _ string) ([]*transcript.Playlist, error) {
	return f.playlists, f.listErr
}
func (f *fakeFacade) GetPlaylist(_ context.Context, _ *mongo.Client, _ string) (*transcript.Playlist, error) {
	return nil, errors.New("not found")
}
func (f *fakeFacade) InsertPlaylist(_ context.Context, _ *mongo.Client, _ *transcript.Playlist) error {
	return nil
}
func (f *fakeFacade) UpdatePlaylist(_ context.Context, _ *mongo.Client, p *transcript.Playlist) error {
	snap := *p
	f.updated = append(f.updated, &snap)
	return nil
}
func (f *fakeFacade) UpsertPlaylist(_ context.Context, _ *mongo.Client, _ *transcript.Playlist) error {
	return nil
}
func (f *fakeFacade) DeletePlaylist(_ context.Context, _ *mongo.Client, _ string) error { return nil }
func (f *fakeFacade) ListAllVideos(_ context.Context, _ *mongo.Client) ([]*transcript.Video, error) {
	return nil, nil
}
func (f *fakeFacade) ListVideos(_ context.Context, _ *mongo.Client, _ string) ([]*transcript.Video, error) {
	return nil, nil
}
func (f *fakeFacade) GetVideo(_ context.Context, _ *mongo.Client, videoID string) (*transcript.Video, error) {
	v, ok := f.videos[videoID]
	if !ok || v == nil {
		return nil, errors.New("not found")
	}
	return v, nil
}
func (f *fakeFacade) InsertVideo(_ context.Context, _ *mongo.Client, _ *transcript.Video) error {
	return nil
}
func (f *fakeFacade) UpdateVideo(_ context.Context, _ *mongo.Client, _ *transcript.Video) error {
	return nil
}
func (f *fakeFacade) DeleteVideo(_ context.Context, _ *mongo.Client, _ string) error { return nil }
func (f *fakeFacade) ListTranscripts(_ context.Context, _ *mongo.Client, _ string) ([]*transcript.Transcript, error) {
	return nil, nil
}
func (f *fakeFacade) GetTranscript(_ context.Context, _ *mongo.Client, _, _ string) (*transcript.Transcript, error) {
	return nil, errors.New("not found")
}
func (f *fakeFacade) InsertTranscript(_ context.Context, _ *mongo.Client, _ *transcript.Transcript) error {
	return nil
}
func (f *fakeFacade) UpdateTranscript(_ context.Context, _ *mongo.Client, _ *transcript.Transcript) error {
	return nil
}
func (f *fakeFacade) DeleteTranscript(_ context.Context, _ *mongo.Client, _ string) error {
	return nil
}

// injectDeps replaces the package-level injectable vars with fakes and returns
// a restore function. The caller must defer the restore.
func injectDeps(t *testing.T, fake *fakeFacade, entries []*transcript.PlaylistEntry, pipelineErr func(videoID string) error) func() {
	t.Helper()
	origFacade := newFacadeFn
	origClient := newDBClientFn
	origScan := scanPlaylistFn
	origPipeline := runVideoPipelineFn
	origWriteAll := writeAllMarkdownFn

	newFacadeFn = func() db.Facade { return fake }
	newDBClientFn = func() (*mongo.Client, error) { return nil, nil }
	scanPlaylistFn = func(_, _ string, _ int64) ([]*transcript.PlaylistEntry, error) {
		return entries, nil
	}
	runVideoPipelineFn = func(_ context.Context, _ db.Facade, _ *mongo.Client, v *transcript.Video) error {
		return pipelineErr(v.VideoId)
	}
	writeAllMarkdownFn = func(_ context.Context, _ db.Facade, _ *mongo.Client) error { return nil }

	return func() {
		newFacadeFn = origFacade
		newDBClientFn = origClient
		scanPlaylistFn = origScan
		runVideoPipelineFn = origPipeline
		writeAllMarkdownFn = origWriteAll
	}
}

// --- Handler-level (HTTP) tests ---

func TestHandler_GET_EchoesChallenge(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?hub.challenge=abc123", nil)
	w := httptest.NewRecorder()
	Handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "abc123" {
		t.Errorf("body = %q, want %q", got, "abc123")
	}
}

func TestHandler_GET_EmptyChallenge(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "" {
		t.Errorf("body = %q, want empty", got)
	}
}

func TestHandler_POST_InvalidXML(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not valid xml"))
	w := httptest.NewRecorder()
	Handler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_POST_MissingChannelID(t *testing.T) {
	body := `<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015">` +
		`<entry><yt:videoId>abc</yt:videoId><title>No Channel</title></entry>` +
		`</feed>`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	Handler(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandler_POST_PassesChannelIDToPipeline(t *testing.T) {
	orig := runPipelineFn
	defer func() { runPipelineFn = orig }()

	var gotChannelID string
	runPipelineFn = func(_ context.Context, channelID string) error {
		gotChannelID = channelID
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validFeedXML("vid1", "chan1")))
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if gotChannelID != "chan1" {
		t.Errorf("channelID = %q, want %q", gotChannelID, "chan1")
	}
}

func TestHandler_POST_PipelineError_StillReturns204(t *testing.T) {
	orig := runPipelineFn
	defer func() { runPipelineFn = orig }()

	runPipelineFn = func(_ context.Context, _ string) error {
		return errors.New("pipeline failed")
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validFeedXML("vid1", "chan1")))
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/", nil)
		w := httptest.NewRecorder()
		Handler(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
		}
	}
}

// --- runPipeline business-logic tests ---

func TestRunPipeline_NoPlaylists(t *testing.T) {
	fake := newFake(nil)
	restore := injectDeps(t, fake, nil, func(_ string) error { return nil })
	defer restore()

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunPipeline_SkipsExistingVideos(t *testing.T) {
	pl := &transcript.Playlist{PlaylistId: "pl1"}
	fake := newFake([]*transcript.Playlist{pl})
	fake.videos["v1"] = &transcript.Video{VideoId: "v1"} // already in DB

	pipelineCalled := false
	restore := injectDeps(t, fake, []*transcript.PlaylistEntry{makeEntry("v1", "", 0, time.Now())},
		func(_ string) error {
			pipelineCalled = true
			return nil
		})
	defer restore()

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Fatal(err)
	}
	if pipelineCalled {
		t.Error("pipeline.Run called for video already in DB")
	}
}

func TestRunPipeline_UpdatesPlaylistOnSuccess(t *testing.T) {
	pl := &transcript.Playlist{PlaylistId: "pl1", NumEntries: 0}
	fake := newFake([]*transcript.Playlist{pl})

	restore := injectDeps(t, fake, []*transcript.PlaylistEntry{makeEntry("v1", "tok1", 4, time.Now())},
		func(_ string) error { return nil })
	defer restore()

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Fatal(err)
	}
	if len(fake.updated) != 1 {
		t.Fatalf("UpdatePlaylist called %d times, want 1", len(fake.updated))
	}
	got := fake.updated[0]
	if got.PageToken != "tok1" {
		t.Errorf("pageToken = %q, want %q", got.PageToken, "tok1")
	}
	if got.NumEntries != 5 { // position 4 → numEntries = 5
		t.Errorf("numEntries = %d, want 5", got.NumEntries)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("updatedAt not set after successful pipeline run")
	}
}

func TestRunPipeline_DoesNotDecreaseNumEntries(t *testing.T) {
	pl := &transcript.Playlist{PlaylistId: "pl1", NumEntries: 10}
	fake := newFake([]*transcript.Playlist{pl})

	// Video at position 2 — lower than existing numEntries of 10.
	restore := injectDeps(t, fake, []*transcript.PlaylistEntry{makeEntry("v1", "tok1", 2, time.Now())},
		func(_ string) error { return nil })
	defer restore()

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Fatal(err)
	}
	if fake.updated[0].NumEntries != 10 {
		t.Errorf("numEntries = %d, want 10 (should not decrease)", fake.updated[0].NumEntries)
	}
}

func TestRunPipeline_CircuitBreakerStopsAllProcessing(t *testing.T) {
	t.Setenv("MAX_PIPELINE_FAILURES", "3")

	playlists := []*transcript.Playlist{
		{PlaylistId: "pl1"},
		{PlaylistId: "pl2"},
	}
	fake := newFake(playlists)

	entries := []*transcript.PlaylistEntry{
		makeEntry("v1", "", 0, time.Now()),
		makeEntry("v2", "", 1, time.Now()),
		makeEntry("v3", "", 2, time.Now()),
		makeEntry("v4", "", 3, time.Now()),
	}

	scanned := []string{}
	origScan := scanPlaylistFn
	defer func() { scanPlaylistFn = origScan }()
	scanPlaylistFn = func(playlistId, _ string, _ int64) ([]*transcript.PlaylistEntry, error) {
		scanned = append(scanned, playlistId)
		return entries, nil
	}

	origFacade := newFacadeFn
	defer func() { newFacadeFn = origFacade }()
	newFacadeFn = func() db.Facade { return fake }

	origClient := newDBClientFn
	defer func() { newDBClientFn = origClient }()
	newDBClientFn = func() (*mongo.Client, error) { return nil, nil }

	called := 0
	origPipeline := runVideoPipelineFn
	defer func() { runVideoPipelineFn = origPipeline }()
	runVideoPipelineFn = func(_ context.Context, _ db.Facade, _ *mongo.Client, _ *transcript.Video) error {
		called++
		return errors.New("pipeline error")
	}

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Fatal(err)
	}

	// Exactly 3 calls before circuit trips.
	if called != 3 {
		t.Errorf("pipeline called %d times, want 3", called)
	}
	// pl2 must not have been scanned.
	if len(scanned) != 1 || scanned[0] != "pl1" {
		t.Errorf("scanned = %v, want [pl1]", scanned)
	}
	if len(fake.updated) != 0 {
		t.Errorf("UpdatePlaylist called %d times, want 0", len(fake.updated))
	}
}

func TestRunPipeline_PipelineErrorLogsAndContinues(t *testing.T) {
	pl := &transcript.Playlist{PlaylistId: "pl1"}
	fake := newFake([]*transcript.Playlist{pl})

	entries := []*transcript.PlaylistEntry{
		makeEntry("v1", "tok1", 0, time.Now()),
		makeEntry("v2", "tok1", 1, time.Now()),
	}
	// v1 fails, v2 succeeds.
	restore := injectDeps(t, fake, entries, func(videoID string) error {
		if videoID == "v1" {
			return errors.New("transient error")
		}
		return nil
	})
	defer restore()

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Fatal(err)
	}
	// v2 succeeded, so UpdatePlaylist should have been called once.
	if len(fake.updated) != 1 {
		t.Errorf("UpdatePlaylist called %d times, want 1", len(fake.updated))
	}
	if fake.updated[0].PageToken != "tok1" {
		t.Errorf("pageToken = %q, want %q", fake.updated[0].PageToken, "tok1")
	}
}

func TestRunPipeline_UsesStoredPageTokenAboveThreshold(t *testing.T) {
	t.Setenv("PLAYLIST_SCAN_THRESHOLD", "100")
	pl := &transcript.Playlist{
		PlaylistId: "pl1",
		NumEntries: 150,
		PageToken:  "stored-token",
	}
	fake := newFake([]*transcript.Playlist{pl})

	var gotStartToken string
	origScan := scanPlaylistFn
	defer func() { scanPlaylistFn = origScan }()
	scanPlaylistFn = func(_, startToken string, _ int64) ([]*transcript.PlaylistEntry, error) {
		gotStartToken = startToken
		return nil, nil
	}

	origFacade := newFacadeFn
	defer func() { newFacadeFn = origFacade }()
	newFacadeFn = func() db.Facade { return fake }

	origClient := newDBClientFn
	defer func() { newDBClientFn = origClient }()
	newDBClientFn = func() (*mongo.Client, error) { return nil, nil }

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Fatal(err)
	}
	if gotStartToken != "stored-token" {
		t.Errorf("startToken = %q, want %q", gotStartToken, "stored-token")
	}
}

func TestRunPipeline_StartsFromBeginningBelowThreshold(t *testing.T) {
	t.Setenv("PLAYLIST_SCAN_THRESHOLD", "100")
	pl := &transcript.Playlist{
		PlaylistId: "pl1",
		NumEntries: 50,
		PageToken:  "stored-token",
	}
	fake := newFake([]*transcript.Playlist{pl})

	var gotStartToken string
	origScan := scanPlaylistFn
	defer func() { scanPlaylistFn = origScan }()
	scanPlaylistFn = func(_, startToken string, _ int64) ([]*transcript.PlaylistEntry, error) {
		gotStartToken = startToken
		return nil, nil
	}

	origFacade := newFacadeFn
	defer func() { newFacadeFn = origFacade }()
	newFacadeFn = func() db.Facade { return fake }

	origClient := newDBClientFn
	defer func() { newDBClientFn = origClient }()
	newDBClientFn = func() (*mongo.Client, error) { return nil, nil }

	if err := runPipeline(context.Background(), "chan1"); err != nil {
		t.Fatal(err)
	}
	if gotStartToken != "" {
		t.Errorf("startToken = %q, want empty (below threshold)", gotStartToken)
	}
}
