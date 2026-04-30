package officials

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// llmModel is the OpenAI model used for the LLM extractor. Matches the model
// already used by pkg/summarize so we don't fan out model dependencies.
const llmModel = openai.GPT4Dot1Mini

// chatClient is the subset of the go-openai client surface we depend on, so
// tests can inject a stub without standing up a real HTTP server.
type chatClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// LLMExtractor implements Option B: feed the page HTML to an LLM and ask for
// a JSON list of committees and members. Use NewLLMExtractor unless you're
// injecting a stub client in a test.
type LLMExtractor struct {
	Client chatClient
	Model  string
}

// NewLLMExtractor returns an LLMExtractor configured with a real OpenAI
// client backed by the CHATGPT_API_KEY env var (matching the convention used
// elsewhere in this repo). Returns an error if the key is missing.
func NewLLMExtractor() (*LLMExtractor, error) {
	apiKey := os.Getenv("CHATGPT_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("officials: CHATGPT_API_KEY not set")
	}
	return &LLMExtractor{
		Client: openai.NewClient(apiKey),
		Model:  llmModel,
	}, nil
}

// llmResponse is the JSON shape the model is asked to return.
type llmResponse struct {
	Committees []struct {
		Name    string   `json:"name"`
		Members []string `json:"members"`
	} `json:"committees"`
}

const llmSystemPrompt = `You extract appointed officials from a Milton, MA town website page.

The page is organized into tabbed sections; each tab is a committee or board (e.g. "Board of Assessors", "School Committee", "Town Moderator"). Within each section, member names are presented either as headings inside a CityDirectory widget or as plain headings.

Return JSON only, matching this schema exactly:
{ "committees": [ { "name": string, "members": [string, ...] }, ... ] }

Rules:
- Include every committee/section that contains at least one named person.
- "members" must contain only people's names (no titles, roles, emails, or "Email …" link text).
- Preserve names exactly as written, including middle initials, suffixes, and order. If a name appears more than once on the page, include it more than once.
- Do not invent committees or members. Do not deduplicate. Do not reorder.
- Output JSON only — no prose, no markdown, no code fences.`

// ParseTownWideLLM extracts committees from the supplied HTML using the LLM.
// Section names are normalized identically to the DOM parser; member names
// have whitespace collapsed but otherwise reflect what the model returned.
func (e *LLMExtractor) ParseTownWideLLM(ctx context.Context, htmlBody string) ([]Committee, error) {
	if e == nil || e.Client == nil {
		return nil, fmt.Errorf("officials: LLMExtractor not configured")
	}
	model := e.Model
	if model == "" {
		model = llmModel
	}

	resp, err := e.Client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: llmSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: htmlBody},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("officials: chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("officials: chat completion returned no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	var parsed llmResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("officials: decode llm json: %w", err)
	}

	out := make([]Committee, 0, len(parsed.Committees))
	for _, c := range parsed.Committees {
		members := make([]string, 0, len(c.Members))
		for _, m := range c.Members {
			if m = collapseWhitespace(m); m != "" {
				members = append(members, m)
			}
		}
		out = append(out, Committee{
			Name:    normalizeCommitteeName(c.Name),
			Members: members,
		})
	}
	return out, nil
}
