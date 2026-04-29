package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// validFeedXML builds a minimal YouTube PubSubHubbub Atom payload.
func validFeedXML(videoID string) string {
	return `<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015">` +
		`<entry><yt:videoId>` + videoID + `</yt:videoId><title>Test Video</title></entry>` +
		`</feed>`
}

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
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not valid xml at all"))
	w := httptest.NewRecorder()

	Handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_POST_MissingVideoID(t *testing.T) {
	body := `<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015"><entry><title>No ID</title></entry></feed>`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	Handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_POST_PipelineSuccess(t *testing.T) {
	orig := runPipelineFn
	defer func() { runPipelineFn = orig }()

	var gotVideoID string
	runPipelineFn = func(_ context.Context, videoID string) error {
		gotVideoID = videoID
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validFeedXML("xyz789")))
	w := httptest.NewRecorder()

	Handler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if gotVideoID != "xyz789" {
		t.Errorf("pipeline called with videoID %q, want %q", gotVideoID, "xyz789")
	}
}

func TestHandler_POST_PipelineError(t *testing.T) {
	orig := runPipelineFn
	defer func() { runPipelineFn = orig }()

	runPipelineFn = func(_ context.Context, _ string) error {
		return errors.New("pipeline failed")
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validFeedXML("xyz789")))
	w := httptest.NewRecorder()

	Handler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
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
