package transcript

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetId(t *testing.T) {
	tests := []struct {
		name         string
		videoId      string
		languageCode string
		wantErr      bool
		wantId       string
	}{
		{
			name:         "valid inputs produce combined id",
			videoId:      "abc123",
			languageCode: "en",
			wantId:       "abc123_en",
		},
		{
			name:         "empty videoId returns error",
			videoId:      "",
			languageCode: "en",
			wantErr:      true,
		},
		{
			name:         "empty languageCode returns error",
			videoId:      "abc123",
			languageCode: "",
			wantErr:      true,
		},
		{
			name:         "both empty returns error",
			videoId:      "",
			languageCode: "",
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Transcript{VideoId: tt.videoId, LanguageCode: tt.languageCode}
			err := tr.SetId()
			if (err != nil) != tt.wantErr {
				t.Errorf("SetId() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tr.Id != tt.wantId {
				t.Errorf("SetId() Id = %q, want %q", tr.Id, tt.wantId)
			}
		})
	}
}

func TestNewTranscript(t *testing.T) {
	tr := NewTranscript("vid1", "en", "some text")
	if tr.VideoId != "vid1" {
		t.Errorf("VideoId = %q, want %q", tr.VideoId, "vid1")
	}
	if tr.LanguageCode != "en" {
		t.Errorf("LanguageCode = %q, want %q", tr.LanguageCode, "en")
	}
	if tr.RetrievedText != "some text" {
		t.Errorf("RetrievedText = %q, want %q", tr.RetrievedText, "some text")
	}
	if tr.Id != "vid1_en" {
		t.Errorf("Id = %q, want %q", tr.Id, "vid1_en")
	}
}

func TestNewTranscriptEmptyFields(t *testing.T) {
	tr := NewTranscript("", "en", "text")
	// SetId fails silently (returns error); Id should remain empty
	if tr.Id != "" {
		t.Errorf("expected empty Id when videoId is empty, got %q", tr.Id)
	}
}

func TestDecodeDoubleEncodedString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"no entities here", "no entities here"},
		{"", ""},
		{"Tom &amp; Jerry", "Tom & Jerry"},
		{"&lt;tag&gt;", "<tag>"},
		{"&quot;quoted&quot;", `"quoted"`},
		{"&apos;apostrophe&apos;", "'apostrophe'"},
		{"&#39;tick&#39;", "'tick'"},
		{"&#34;double&#34;", `"double"`},
		{"&amp;&amp;", "&&"},
		{"no ampersand", "no ampersand"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := decodeDoubleEncodedString(tt.input)
			if got != tt.want {
				t.Errorf("decodeDoubleEncodedString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCaptionDownload(t *testing.T) {
	xmlBody := `<transcript><text>Hello world</text><text>Goodbye</text></transcript>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(xmlBody))
	}))
	defer server.Close()

	c := &Caption{BaseUrl: server.URL, LanguageCode: "en"}
	resp, err := c.Download()
	if err != nil {
		t.Fatalf("Download() unexpected error: %v", err)
	}
	if len(resp.Text) != 2 {
		t.Fatalf("Download() len(Text) = %d, want 2", len(resp.Text))
	}
	if resp.Text[0] != "Hello world" {
		t.Errorf("Text[0] = %q, want %q", resp.Text[0], "Hello world")
	}
	if resp.Text[1] != "Goodbye" {
		t.Errorf("Text[1] = %q, want %q", resp.Text[1], "Goodbye")
	}
}

func TestCaptionDownloadDecodesEntities(t *testing.T) {
	// YouTube caption XML contains double-encoded HTML entities.
	// &amp;amp; → XML decodes to &amp; → decodeDoubleEncodedString decodes to &
	xmlBody := `<transcript><text>Tom &amp;amp; Jerry</text></transcript>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xmlBody))
	}))
	defer server.Close()

	c := &Caption{BaseUrl: server.URL, LanguageCode: "en"}
	resp, err := c.Download()
	if err != nil {
		t.Fatalf("Download() unexpected error: %v", err)
	}
	if len(resp.Text) != 1 {
		t.Fatalf("len(Text) = %d, want 1", len(resp.Text))
	}
	if resp.Text[0] != "Tom & Jerry" {
		t.Errorf("Text[0] = %q, want %q", resp.Text[0], "Tom & Jerry")
	}
}

func TestCaptionDownloadServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := &Caption{BaseUrl: server.URL, LanguageCode: "en"}
	resp, err := c.Download()
	// An HTTP 500 with an empty body produces invalid XML; Download returns an error
	// or an empty CaptionResponse (behaviour depends on xml.Unmarshal with empty input).
	// Either outcome is acceptable — we just must not panic.
	if err == nil && resp != nil && len(resp.Text) != 0 {
		t.Errorf("expected empty Text for 500 response with empty body, got %v", resp.Text)
	}
}

func TestCaptionDownloadUnreachable(t *testing.T) {
	c := &Caption{BaseUrl: "http://127.0.0.1:0", LanguageCode: "en"}
	_, err := c.Download()
	if err == nil {
		t.Error("Download() expected error for unreachable server, got nil")
	}
}

func TestCaptionExtractText(t *testing.T) {
	xmlBody := `<transcript><text>First</text><text>Second</text><text>Third</text></transcript>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xmlBody))
	}))
	defer server.Close()

	c := &Caption{BaseUrl: server.URL, LanguageCode: "en"}
	text, err := c.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText() unexpected error: %v", err)
	}
	want := "First Second Third "
	if text != want {
		t.Errorf("ExtractText() = %q, want %q", text, want)
	}
}

func TestCaptionExtractTextEmpty(t *testing.T) {
	xmlBody := `<transcript></transcript>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xmlBody))
	}))
	defer server.Close()

	c := &Caption{BaseUrl: server.URL, LanguageCode: "en"}
	text, err := c.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText() unexpected error: %v", err)
	}
	if text != "" {
		t.Errorf("ExtractText() = %q, want empty string", text)
	}
}
