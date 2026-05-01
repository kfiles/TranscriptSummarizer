package facebook

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	md "github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
)

// fbRenderer converts Markdown to Facebook-friendly plain text.
// Headings become ━━━ HEADING ━━━ section dividers; list items use • bullets.
type fbRenderer struct{}

func (r *fbRenderer) RenderNode(w io.Writer, node ast.Node, entering bool) ast.WalkStatus {
	switch n := node.(type) {
	case *ast.Heading:
		if entering {
			w.Write([]byte("\n━━━ "))
		} else {
			w.Write([]byte(" ━━━\n"))
		}
	case *ast.Text:
		w.Write(n.Literal)
	case *ast.Paragraph:
		if !entering {
			w.Write([]byte("\n"))
		}
	case *ast.ListItem:
		if entering {
			w.Write([]byte("• "))
		} else {
			w.Write([]byte("\n"))
		}
	case *ast.Softbreak, *ast.Hardbreak, *ast.NonBlockingSpace:
		w.Write([]byte(" "))
	case *ast.HorizontalRule:
		w.Write([]byte("\n"))
	}
	return ast.GoToNext
}

func (r *fbRenderer) RenderHeader(w io.Writer, _ ast.Node) {}
func (r *fbRenderer) RenderFooter(w io.Writer, _ ast.Node) {}

// TranscriptURL returns the Firebase Hosting URL for a video's transcript page.
func TranscriptURL(projectID, videoID string, publishedAt time.Time) string {
	return fmt.Sprintf("https://%s.web.app/minutes/%s/%s/%s/",
		projectID,
		publishedAt.Format("2006"),
		publishedAt.Format("January"),
		videoID)
}

// FormatPost converts a Markdown summary to a Facebook-ready plain-text post.
// If transcriptURL is non-empty it is appended as a plain URL (auto-linkified by Facebook).
func FormatPost(title, markdownSummary, transcriptURL string) string {
	p := parser.New()
	doc := md.Parse([]byte(markdownSummary), p)
	rendered := md.Render(doc, &fbRenderer{})
	body := strings.TrimSpace(string(rendered))
	post := title + "\n\n" + body
	if transcriptURL != "" {
		post += "\n\nFull transcript: " + transcriptURL
	}
	return post
}

// postToEndpoint sends the message to an arbitrary feed endpoint — used directly by tests.
func postToEndpoint(endpoint, token, message string) error {
	resp, err := http.PostForm(endpoint, url.Values{
		"message":      {message},
		"access_token": {token},
	})
	if err != nil {
		return fmt.Errorf("facebook post request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("facebook API error %d: %s", resp.StatusCode, body)
	}
	return nil
}

// PostToPage publishes a message to a Facebook Page via the Graph API.
func PostToPage(pageID, token, message string) error {
	endpoint := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/feed", pageID)
	return postToEndpoint(endpoint, token, message)
}
