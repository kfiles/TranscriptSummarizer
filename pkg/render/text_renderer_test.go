package render

import (
	"strings"
	"testing"
)

func TestSanitizeTextOfficialMinutes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Official Minutes of the meeting", "Unofficial Minutes of the meeting"},
		{"official minutes of the meeting", "Unofficial Minutes of the meeting"},
		{"These are Official Minutes", "These are Official Minutes"}, // only replaces at start of string
	}
	for _, tt := range tests {
		got := sanitizeText(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeTextMinutesPrepared(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Minutes Prepared by John Smith", ""},
		{"minutes prepared by Jane Doe on April 1", ""},
		{"The minutes prepared here", "The minutes prepared here"}, // not at start
	}
	for _, tt := range tests {
		got := sanitizeText(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeTextPlaceholder(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"[Your Name], Town Reporter", ""},
		{"  [Your name] here", ""},
		{"Contact [Your Name] for info", "Contact [Your Name] for info"}, // not at start of string
		// The regex requires a capital 'Y', so all-lowercase does not match.
		{"[your name]", "[your name]"},
	}
	for _, tt := range tests {
		got := sanitizeText(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeTextNoMatch(t *testing.T) {
	input := "Regular meeting notes from April."
	got := sanitizeText(input)
	if got != input {
		t.Errorf("sanitizeText(%q) = %q, want unchanged", input, got)
	}
}

func TestToPlainTextHeadingAndParagraph(t *testing.T) {
	md := "# Attendance\n\nJohn, Jane, Bob\n"
	got := string(ToPlainText(md))
	if !strings.Contains(got, "Attendance") {
		t.Errorf("ToPlainText() missing heading text, got %q", got)
	}
	if !strings.Contains(got, "John, Jane, Bob") {
		t.Errorf("ToPlainText() missing paragraph text, got %q", got)
	}
	if strings.Contains(got, "#") {
		t.Errorf("ToPlainText() should strip markdown heading markers, got %q", got)
	}
}

func TestToPlainTextBulletList(t *testing.T) {
	md := "- Item one\n- Item two\n- Item three\n"
	got := string(ToPlainText(md))
	if !strings.Contains(got, "•") {
		t.Errorf("ToPlainText() should convert list items to bullets, got %q", got)
	}
	if !strings.Contains(got, "Item one") {
		t.Errorf("ToPlainText() missing list item text, got %q", got)
	}
	if strings.Contains(got, "-") {
		t.Errorf("ToPlainText() should strip markdown list markers, got %q", got)
	}
}

func TestToPlainTextSanitizesOfficialMinutes(t *testing.T) {
	md := "## Official Minutes\n\nSome content.\n"
	got := string(ToPlainText(md))
	if strings.Contains(got, "Official Minutes") {
		t.Errorf("ToPlainText() should replace 'Official Minutes', got %q", got)
	}
	if !strings.Contains(got, "Unofficial Minutes") {
		t.Errorf("ToPlainText() should produce 'Unofficial Minutes', got %q", got)
	}
}

func TestToPlainTextSanitizesMinutesPrepared(t *testing.T) {
	md := "Minutes Prepared by Jane Doe\n"
	got := string(ToPlainText(md))
	if strings.Contains(got, "Minutes Prepared") {
		t.Errorf("ToPlainText() should remove 'Minutes Prepared' lines, got %q", got)
	}
}

func TestToPlainTextEmpty(t *testing.T) {
	got := ToPlainText("")
	if len(got) != 0 {
		t.Errorf("ToPlainText(\"\") = %q, want empty", got)
	}
}

func TestToPlainTextStripsMarkdownFormatting(t *testing.T) {
	md := "**bold** and _italic_ text\n"
	got := string(ToPlainText(md))
	if strings.Contains(got, "**") || strings.Contains(got, "_") {
		t.Errorf("ToPlainText() should strip markdown formatting symbols, got %q", got)
	}
	if !strings.Contains(got, "bold") || !strings.Contains(got, "italic") {
		t.Errorf("ToPlainText() should preserve text content, got %q", got)
	}
}
