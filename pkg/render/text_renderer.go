package render

import (
	md "github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"io"
	"regexp"
)

var officialMinutesRegexp = regexp.MustCompile(`^[Oo]fficial [Mm]inutes`)
var minutesPreparedRegexp = regexp.MustCompile(`^[Mm]inutes [Pp]repared.*`)
var placeholderRegexp = regexp.MustCompile(`^\s*\[Your [Nn]ame\].*`)

type plaintextRenderer struct {
}

func sanitizeText(s string) string {
	s = officialMinutesRegexp.ReplaceAllString(s, "Unofficial Minutes")
	s = minutesPreparedRegexp.ReplaceAllString(s, "")
	s = placeholderRegexp.ReplaceAllString(s, "")
	return s
}

func (r *plaintextRenderer) Text(w io.Writer, text *ast.Text) {
	clean := sanitizeText(string(text.Literal))
	w.Write([]byte(clean))
}

func (r *plaintextRenderer) ListItem(w io.Writer, text *ast.ListItem, entering bool) {
	if entering {
		w.Write([]byte(" • "))
	}
}

func (r *plaintextRenderer) Whitespace(w io.Writer) {
	w.Write([]byte(" "))
}

func (r *plaintextRenderer) Newline(w io.Writer) {
	w.Write([]byte("\n"))
}
func (r *plaintextRenderer) RenderNode(w io.Writer, node ast.Node, entering bool) ast.WalkStatus {
	switch node := node.(type) {
	case *ast.Text:
		r.Text(w, node)
	case *ast.Softbreak:
		r.Whitespace(w)
	case *ast.Hardbreak:
		r.Whitespace(w)
	case *ast.NonBlockingSpace:
		r.Whitespace(w)
	case *ast.BlockQuote:
		w.Write([]byte("\n\"\n"))
	case *ast.Callout:
		w.Write([]byte("\n\"\n"))
	case *ast.Paragraph:
		if !entering {
			r.Newline(w)
		}
	case *ast.Heading:
		r.Newline(w)
	case *ast.HTMLSpan:
		r.Whitespace(w)
	case *ast.HorizontalRule:
		r.Newline(w)
	case *ast.ListItem:
		r.ListItem(w, node, entering)
	default:
		{
		}
	}
	return ast.GoToNext
}

func (r *plaintextRenderer) RenderHeader(w io.Writer, ast ast.Node) {
}

func (r *plaintextRenderer) RenderFooter(w io.Writer, ast ast.Node) {

}

func ToPlainText(markdown string) []byte {
	p := parser.New()
	doc := md.Parse([]byte(markdown), p)
	renderer := &plaintextRenderer{}
	return md.Render(doc, renderer)
}
