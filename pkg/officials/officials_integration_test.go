//go:build integration

package officials

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestIntegrationTownWideDOM hits the live Town-Wide page and asserts that
// the DOM parser returns a structurally sane result: at least 13 committees,
// at least 40 members in total, and a handful of sentinel names that we know
// are stable across routine roster changes.
func TestIntegrationTownWideDOM(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	html, err := Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	got, err := ParseTownWideDOM(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseTownWideDOM: %v", err)
	}

	assertDOMSentinels(t, got)
}

// TestIntegrationTownWideLLM hits the live page and the OpenAI API, then
// asserts that the LLM output agrees with the DOM parser output (modulo
// ordering and minor whitespace). Skipped unless CHATGPT_API_KEY is set.
func TestIntegrationTownWideLLM(t *testing.T) {
	if os.Getenv("CHATGPT_API_KEY") == "" {
		t.Skip("set CHATGPT_API_KEY to run LLM integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	html, err := Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	domResult, err := ParseTownWideDOM(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseTownWideDOM: %v", err)
	}

	ex, err := NewLLMExtractor()
	if err != nil {
		t.Fatalf("NewLLMExtractor: %v", err)
	}
	llmResult, err := ex.ParseTownWideLLM(ctx, html)
	if err != nil {
		t.Fatalf("ParseTownWideLLM: %v", err)
	}

	assertDOMSentinels(t, llmResult)
	assertResultsAgree(t, domResult, llmResult)
}

// assertDOMSentinels checks structural and sentinel invariants on any result,
// DOM or LLM.
func assertDOMSentinels(t *testing.T, committees []Committee) {
	t.Helper()

	if len(committees) < 13 {
		t.Errorf("got %d committees, want ≥ 13", len(committees))
	}

	total := 0
	for _, c := range committees {
		total += len(c.Members)
	}
	if total < 40 {
		t.Errorf("got %d total members, want ≥ 40", total)
	}

	// Sentinel: name → expected normalized committee name.
	sentinels := map[string]string{
		"Marybeth Joyce":  "Blue Hills Regional Technical School",
		"Susan M. Galvin": "Town Clerk",
	}
	byName := make(map[string]Committee, len(committees))
	for _, c := range committees {
		byName[c.Name] = c
	}
	for member, committee := range sentinels {
		c, ok := byName[committee]
		if !ok {
			t.Errorf("committee %q not found; have: %v", committee, committeeNames(committees))
			continue
		}
		if !containsMember(c.Members, member) {
			t.Errorf("committee %q: member %q not found; have: %v", committee, member, c.Members)
		}
	}
}

// assertResultsAgree checks that the LLM output matches the DOM output in all
// meaningful ways: same set of committee names, and within each committee, the
// same set of member names. Order is not asserted because the LLM may return
// committees in a different sequence.
func assertResultsAgree(t *testing.T, dom, llm []Committee) {
	t.Helper()

	domMap := committeeMap(dom)
	llmMap := committeeMap(llm)

	// Every committee the DOM found should appear in the LLM output.
	for name, domMembers := range domMap {
		llmMembers, ok := llmMap[name]
		if !ok {
			t.Errorf("LLM missing committee %q (DOM found it with %d members)", name, len(domMembers))
			continue
		}
		domSet := memberSet(domMembers)
		llmSet := memberSet(llmMembers)
		for m := range domSet {
			if !llmSet[m] {
				t.Errorf("committee %q: DOM member %q absent from LLM output", name, m)
			}
		}
		for m := range llmSet {
			if !domSet[m] {
				t.Errorf("committee %q: LLM member %q not in DOM output (possible hallucination)", name, m)
			}
		}
	}

	// Flag any committees the LLM invented that the DOM did not find.
	for name := range llmMap {
		if _, ok := domMap[name]; !ok {
			t.Errorf("LLM returned committee %q not found by DOM parser", name)
		}
	}
}

func committeeMap(cs []Committee) map[string][]string {
	m := make(map[string][]string, len(cs))
	for _, c := range cs {
		m[c.Name] = c.Members
	}
	return m
}

func memberSet(members []string) map[string]bool {
	s := make(map[string]bool, len(members))
	for _, m := range members {
		s[m] = true
	}
	return s
}

func containsMember(members []string, name string) bool {
	for _, m := range members {
		if m == name {
			return true
		}
	}
	return false
}

func committeeNames(cs []Committee) []string {
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = c.Name
	}
	sort.Strings(names)
	return names
}
