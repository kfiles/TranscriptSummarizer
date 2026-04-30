package officials

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// stubChatClient returns a fixed response or error.
type stubChatClient struct {
	resp openai.ChatCompletionResponse
	err  error

	gotReq openai.ChatCompletionRequest
}

func (s *stubChatClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	s.gotReq = req
	return s.resp, s.err
}

func newStubResponse(content string) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content}},
		},
	}
}

func TestParseTownWideLLM_Success(t *testing.T) {
	stub := &stubChatClient{
		resp: newStubResponse(`{
			"committees": [
				{"name": " school COMMITTEE", "members": ["Elizabeth Marshall Carroll ", "Beverly  Ross Denny"]},
				{"name": "Town Clerk", "members": ["Susan M. Galvin"]}
			]
		}`),
	}
	e := &LLMExtractor{Client: stub}
	got, err := e.ParseTownWideLLM(context.Background(), "<html>...</html>")
	if err != nil {
		t.Fatalf("ParseTownWideLLM: %v", err)
	}
	want := []Committee{
		{Name: "School Committee", Members: []string{"Elizabeth Marshall Carroll", "Beverly Ross Denny"}},
		{Name: "Town Clerk", Members: []string{"Susan M. Galvin"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	// Confirm we requested JSON-mode output and used the configured model.
	if stub.gotReq.ResponseFormat == nil || stub.gotReq.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
		t.Errorf("ResponseFormat = %+v, want json_object", stub.gotReq.ResponseFormat)
	}
	if stub.gotReq.Model != llmModel {
		t.Errorf("Model = %q, want %q", stub.gotReq.Model, llmModel)
	}
	// The HTML body should be passed through verbatim as a user message.
	foundUserHTML := false
	for _, m := range stub.gotReq.Messages {
		if m.Role == openai.ChatMessageRoleUser && strings.Contains(m.Content, "<html>") {
			foundUserHTML = true
		}
	}
	if !foundUserHTML {
		t.Errorf("HTML body not forwarded as user message: %+v", stub.gotReq.Messages)
	}
}

func TestParseTownWideLLM_MalformedJSON(t *testing.T) {
	stub := &stubChatClient{resp: newStubResponse("not json")}
	e := &LLMExtractor{Client: stub}
	if _, err := e.ParseTownWideLLM(context.Background(), ""); err == nil {
		t.Errorf("expected error for malformed JSON, got nil")
	}
}

func TestParseTownWideLLM_NoChoices(t *testing.T) {
	stub := &stubChatClient{resp: openai.ChatCompletionResponse{}}
	e := &LLMExtractor{Client: stub}
	if _, err := e.ParseTownWideLLM(context.Background(), ""); err == nil {
		t.Errorf("expected error for empty Choices, got nil")
	}
}

func TestParseTownWideLLM_ClientError(t *testing.T) {
	stub := &stubChatClient{err: errors.New("boom")}
	e := &LLMExtractor{Client: stub}
	if _, err := e.ParseTownWideLLM(context.Background(), ""); err == nil {
		t.Errorf("expected error from client, got nil")
	}
}

func TestParseTownWideLLM_NilExtractor(t *testing.T) {
	if _, err := (*LLMExtractor)(nil).ParseTownWideLLM(context.Background(), ""); err == nil {
		t.Errorf("expected error for nil extractor, got nil")
	}
}
