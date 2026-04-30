package summarize

import (
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/net/context"
	"os"
)

const userPromptBase = `Summarize the transcript of the meeting below. The title of the minutes must include the phrase "Unofficial Minutes". Include information about attendance, what topics were discussed, what motions were made, and whether they passed. Generate in valid Markdown format.`

func Summarize(ctx context.Context, text string, names []string) (string, error) {
	apiKey := os.Getenv("CHATGPT_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("CHATGPT_API_KEY environment variable not set")
	}

	userPrompt := userPromptBase
	if len(names) > 0 {
		userPrompt += " Ensure that the following names are spelled correctly: " + strings.Join(names, ", ")
	}

	client := openai.NewClient(apiKey)
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "gpt-4.1-mini",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a professional town reporter reporting unofficial minutes of town meetings. Use formal language, titles, and observe all official motions and votes. Organize the meeting into sections by topics of discussion, and use bullet points to summarize.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("ChatCompletion: %w", err)
	}
	return resp.Choices[0].Message.Content, nil
}
