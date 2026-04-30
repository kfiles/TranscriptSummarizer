// Package officials extracts the rosters of Milton, MA appointed officials
// from the town's "Town-Wide" boards & committees page.
//
// Two extraction strategies are provided:
//
//   - ParseTownWideDOM: a deterministic HTML parser that targets the CivicPlus
//     widget classes. This is the default: zero cost, offline-capable, and
//     fails loudly if the page structure changes.
//   - ParseTownWideLLM: a fallback that feeds the page HTML to an LLM and asks
//     for structured JSON. Tolerant of structural drift, but slower and
//     non-deterministic.
package officials

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TownWideURL is the canonical URL for the page this package parses.
const TownWideURL = "https://www.miltonma.gov/890/Town-Wide"

// Committee is a single section of the Town-Wide page with its appointed
// members in document order.
type Committee struct {
	Name    string
	Members []string
}

// Fetch downloads the Town-Wide page HTML using the supplied client, or
// http.DefaultClient when nil.
func Fetch(ctx context.Context, client *http.Client) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TownWideURL, nil)
	if err != nil {
		return "", fmt.Errorf("officials: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; transcriptsummarizer/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("officials: fetch %s: %w", TownWideURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("officials: fetch %s: status %d", TownWideURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("officials: read body: %w", err)
	}
	return string(body), nil
}

// smallWords are connectors that stay lowercase when not the first token.
var smallWords = map[string]bool{
	"of":  true,
	"the": true,
	"and": true,
	"for": true,
	"to":  true,
	"a":   true,
	"an":  true,
	"in":  true,
	"on":  true,
}

// normalizeCommitteeName trims whitespace, collapses runs of internal
// whitespace, and title-cases the section name with a small-word rule so the
// inconsistent capitalization on the source page (e.g. " School COMMITTEE")
// renders cleanly ("School Committee").
func normalizeCommitteeName(s string) string {
	s = collapseWhitespace(s)
	if s == "" {
		return s
	}
	parts := strings.Split(s, " ")
	for i, p := range parts {
		if p == "" {
			continue
		}
		// Only touch tokens that look like words (contain at least one letter).
		// Leaves symbols like "&" alone.
		if !hasLetter(p) {
			continue
		}
		lower := strings.ToLower(p)
		if i > 0 && smallWords[lower] {
			parts[i] = lower
			continue
		}
		parts[i] = capitalizeWord(p)
	}
	return strings.Join(parts, " ")
}

// capitalizeWord uppercases the first letter and lowercases the remainder.
// Handles a leading non-letter (e.g. quote) by capitalizing the first letter
// it finds.
func capitalizeWord(w string) string {
	runes := []rune(strings.ToLower(w))
	for i, r := range runes {
		if r >= 'a' && r <= 'z' {
			runes[i] = r - ('a' - 'A')
			break
		}
	}
	return string(runes)
}

func hasLetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// collapseWhitespace trims s and replaces runs of any whitespace with a
// single ASCII space. Used for both committee and member names so that the
// "Meghan  Haggerty" double-space on the source page becomes "Meghan Haggerty".
func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true // treat start as space so we trim leading whitespace
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ' ' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	out := b.String()
	return strings.TrimRight(out, " ")
}
