package officials

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ParseTownWideDOM parses the Town-Wide page HTML into committees and their
// members using CSS-class selectors specific to the CivicPlus widget. It is
// the default, deterministic extractor.
//
// The parser walks the tabbed widget on the page. For each tab panel it pairs
// the panel with its tab anchor (matching by the trailing "_N" id suffix) to
// pick up the section name, then collects names from one of two layouts:
//
//   - <h4 class="widgetTitle field p-name"> — used by 12 of 13 sections.
//   - <h3 class="subhead2">                — used as a fallback for the
//     Blue Hills Regional Technical School section.
//
// Section names are normalized via normalizeCommitteeName; member names have
// whitespace collapsed but retain their original casing.
func ParseTownWideDOM(r io.Reader) ([]Committee, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("officials: parse html: %w", err)
	}

	tabNames := collectTabNames(doc) // suffix ("_3") -> raw section name
	panels := collectPanels(doc)     // panels in document order

	out := make([]Committee, 0, len(panels))
	for _, p := range panels {
		raw, ok := tabNames[p.suffix]
		if !ok {
			// Panel without a matching tab anchor — skip rather than guess.
			continue
		}
		members := extractMembers(p.node)
		out = append(out, Committee{
			Name:    normalizeCommitteeName(raw),
			Members: members,
		})
	}
	return out, nil
}

type panel struct {
	suffix string // e.g. "_3"
	node   *html.Node
}

// collectPanels returns every <div class="tabbedWidget cpTabPanel ..."> in
// document order, paired with its trailing id suffix.
func collectPanels(root *html.Node) []panel {
	var out []panel
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || n.DataAtom != atom.Div {
			return
		}
		if !classContainsAll(n, "tabbedWidget", "cpTabPanel") {
			return
		}
		id := attr(n, "id")
		idx := strings.LastIndex(id, "_")
		if idx < 0 {
			return
		}
		out = append(out, panel{suffix: id[idx:], node: n})
	})
	return out
}

// collectTabNames returns a map from panel-id suffix (e.g. "_3") to the raw
// section name shown on the tab. We read both the data-tabname attribute and
// the visible text inside <span class="tabName">; data-tabname wins because
// it survives HTML-entity unescaping more cleanly.
func collectTabNames(root *html.Node) map[string]string {
	out := map[string]string{}
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || n.DataAtom != atom.A {
			return
		}
		href := attr(n, "href")
		if !strings.HasPrefix(href, "#tab") {
			return
		}
		idx := strings.LastIndex(href, "_")
		if idx < 0 {
			return
		}
		suffix := href[idx:]
		// Find the inner <span class="tabName ..."> for data-tabname / text.
		var span *html.Node
		walk(n, func(c *html.Node) {
			if span != nil {
				return
			}
			if c.Type == html.ElementNode && c.DataAtom == atom.Span && classContainsAll(c, "tabName") {
				span = c
			}
		})
		if span == nil {
			return
		}
		if name := attr(span, "data-tabname"); name != "" {
			out[suffix] = name
			return
		}
		out[suffix] = textContent(span)
	})
	return out
}

// extractMembers pulls member names from a tab panel. The CityDirectory
// widget uses <h4 class="widgetTitle field p-name"> for each member; some
// sections fall back to <h3 class="subhead2">.
func extractMembers(panelNode *html.Node) []string {
	var primary []string
	var fallback []string
	walk(panelNode, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.DataAtom {
		case atom.H4:
			if classContainsAll(n, "widgetTitle", "field", "p-name") {
				if name := collapseWhitespace(textContent(n)); name != "" {
					primary = append(primary, name)
				}
			}
		case atom.H3:
			if classContainsAll(n, "subhead2") {
				if name := collapseWhitespace(textContent(n)); name != "" {
					fallback = append(fallback, name)
				}
			}
		}
	})
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

// walk invokes fn for n and every descendant in document order.
func walk(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// classContainsAll returns true when every needle appears as a
// whitespace-delimited token in n's class attribute.
func classContainsAll(n *html.Node, needles ...string) bool {
	classes := strings.Fields(attr(n, "class"))
	for _, want := range needles {
		found := false
		for _, c := range classes {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func textContent(n *html.Node) string {
	var b strings.Builder
	walk(n, func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	})
	return b.String()
}
