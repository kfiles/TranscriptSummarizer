package summarize

import (
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/net/context"
	"log"
	"os"
)

func Summarize(ctx context.Context, text string) (string, error) {
	apiKey := os.Getenv("CHATGPT_API_KEY")
	if apiKey == "" {
		log.Fatal("Set your 'CHATGPT_API_KEY' environment variable.")
	}
	// Limit content length for ChatGPT
	//text = string([]rune(text)[:10000])
	client := openai.NewClient(apiKey)
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a professional town reporter reporting unofficial minutes of town meetings. Use formal language, titles, and observe all official motions and votes. Organize the meeting into sections by topics of discussion, and use bullet points to summarize.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Summarize the transcript of the meeting below. Include information about attendance, what topics were discussed, what motions were made, and whether they passed. Generate in valid Markdown format. Ensure that the following names are spelled correctly: John Keohane, Erin Bradley, Richard Wells, Benjamin Zoll, Roxanne Musto",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
		},
	) //Do not add text about the preparation or submission of the minutes.
	//, using headings as appropriate and ordered lists for topics discussed.
	//Use arabic numerals not roman numerals for ordered lists.
	//pay attention to usage of Robert's Rules of Order.
	
	if err != nil {
		return "", fmt.Errorf("ChatCompletion error: %v\n", err)
	}
	return resp.Choices[0].Message.Content, nil
}
