package facebook

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFormatPostTitlePrepended(t *testing.T) {
	got := FormatPost("Board Meeting April 2026", "## Attendance\n\nJohn, Jane\n", "")
	if !strings.HasPrefix(got, "Board Meeting April 2026\n") {
		t.Errorf("FormatPost() title not at start: %q", got)
	}
}

func TestFormatPostHeadingsDividers(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIn  string
		wantOut string
	}{
		{
			name:    "h2 becomes divider",
			input:   "## Attendance\n\nSome text.\n",
			wantIn:  "━━━ Attendance ━━━",
			wantOut: "##",
		},
		{
			name:    "h3 becomes divider",
			input:   "### Votes\n\nMotion passed.\n",
			wantIn:  "━━━ Votes ━━━",
			wantOut: "###",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPost("Title", tt.input, "")
			if !strings.Contains(got, tt.wantIn) {
				t.Errorf("FormatPost() missing %q in output: %q", tt.wantIn, got)
			}
			if strings.Contains(got, tt.wantOut) {
				t.Errorf("FormatPost() should strip markdown heading marker %q, got: %q", tt.wantOut, got)
			}
		})
	}
}

func TestFormatPostBulletItems(t *testing.T) {
	input := "- Motion approved\n- Quorum established\n- Budget reviewed\n"
	got := FormatPost("Title", input, "")
	if !strings.Contains(got, "•") {
		t.Errorf("FormatPost() should convert list items to • bullets, got: %q", got)
	}
	if !strings.Contains(got, "Motion approved") {
		t.Errorf("FormatPost() missing list item text, got: %q", got)
	}
	if strings.Contains(got, "- ") {
		t.Errorf("FormatPost() should strip markdown list markers, got: %q", got)
	}
}

func TestFormatPostEmptySummary(t *testing.T) {
	got := FormatPost("My Title", "", "")
	if !strings.Contains(got, "My Title") {
		t.Errorf("FormatPost() with empty summary should still include title, got: %q", got)
	}
}

func TestFormatPostFullSummary(t *testing.T) {
	input := "## Attendance\n\nJohn Keohane, Erin Bradley\n\n## Motions\n\n- Approve budget\n- Adjourn meeting\n"
	got := FormatPost("April Meeting", input, "")
	if !strings.Contains(got, "━━━ Attendance ━━━") {
		t.Errorf("FormatPost() missing Attendance divider, got: %q", got)
	}
	if !strings.Contains(got, "━━━ Motions ━━━") {
		t.Errorf("FormatPost() missing Motions divider, got: %q", got)
	}
	if !strings.Contains(got, "• Approve budget") {
		t.Errorf("FormatPost() missing bullet item, got: %q", got)
	}
}

func TestFormatPostWithURL(t *testing.T) {
	url := "https://miltonmeetingsummarizer.web.app/minutes/2026/April/abc123/"
	got := FormatPost("April Meeting", "## Section\n\nText.", url)
	if !strings.Contains(got, "Full transcript: "+url) {
		t.Errorf("FormatPost() missing transcript URL line, got: %q", got)
	}
}

func TestFormatPostURLOmittedWhenEmpty(t *testing.T) {
	got := FormatPost("April Meeting", "## Section\n\nText.", "")
	if strings.Contains(got, "Full transcript:") {
		t.Errorf("FormatPost() should not include transcript line when URL is empty, got: %q", got)
	}
}

func TestTranscriptURL(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		videoID     string
		publishedAt time.Time
		want        string
	}{
		{
			name:        "typical April meeting",
			projectID:   "miltonmeetingsummarizer",
			videoID:     "abc123",
			publishedAt: time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC),
			want:        "https://miltonmeetingsummarizer.web.app/minutes/2026/April/abc123/",
		},
		{
			name:        "January year boundary",
			projectID:   "myproject",
			videoID:     "xyz",
			publishedAt: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			want:        "https://myproject.web.app/minutes/2025/January/xyz/",
		},
		{
			name:        "December end of year",
			projectID:   "myproject",
			videoID:     "xyz",
			publishedAt: time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
			want:        "https://myproject.web.app/minutes/2025/December/xyz/",
		},
		{
			name:        "video ID with hyphens",
			projectID:   "civic-app",
			videoID:     "dQw4w9WgXcQ",
			publishedAt: time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC),
			want:        "https://civic-app.web.app/minutes/2026/March/dQw4w9WgXcQ/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranscriptURL(tt.projectID, tt.videoID, tt.publishedAt)
			if got != tt.want {
				t.Errorf("TranscriptURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPostToEndpointSuccess(t *testing.T) {
	var gotMessage, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotMessage = r.FormValue("message")
		gotToken = r.FormValue("access_token")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := postToEndpoint(server.URL, "test-token", "Hello, town!")
	if err != nil {
		t.Fatalf("postToEndpoint() unexpected error: %v", err)
	}
	if gotMessage != "Hello, town!" {
		t.Errorf("postToEndpoint() sent message %q, want %q", gotMessage, "Hello, town!")
	}
	if gotToken != "test-token" {
		t.Errorf("postToEndpoint() sent token %q, want %q", gotToken, "test-token")
	}
}

func TestPostToEndpointAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"bad request", http.StatusBadRequest, `{"error":{"message":"Invalid token"}}`},
		{"server error", http.StatusInternalServerError, `{"error":{"message":"internal error"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			err := postToEndpoint(server.URL, "token", "message")
			if err == nil {
				t.Fatalf("postToEndpoint() expected error for status %d, got nil", tt.statusCode)
			}
			if !strings.Contains(err.Error(), "Invalid token") && !strings.Contains(err.Error(), "internal error") {
				t.Errorf("postToEndpoint() error should include API response body, got: %v", err)
			}
		})
	}
}

func TestPostToEndpointNetworkError(t *testing.T) {
	err := postToEndpoint("http://127.0.0.1:0/feed", "token", "message")
	if err == nil {
		t.Fatal("postToEndpoint() expected error for unreachable server, got nil")
	}
	if !strings.Contains(err.Error(), "facebook post request failed") {
		t.Errorf("postToEndpoint() error should mention request failure, got: %v", err)
	}
}
