package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/huihui4754/expertlib/types"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

var (
	openaiClient openai.Client
)

type ChatAIMessage = types.ChatAIMessage

type LocalLLM struct {
	DialogID            string                                   `json:"dialog_id"`
	AIURL               string                                   `json:"-"`
	AIModel             string                                   `json:"-"`
	SystemPrompt        string                                   `json:"-"`
	MessagesLenLimit    int                                      `json:"-"`
	Messages            []openai.ChatCompletionMessageParamUnion `json:"messages"` // 不包括系统提示词
	LastSavedContentMd5 string                                   `json:"-"`
	Relpying            bool                                     `json:"-"`
}

func (l *LocalLLM) Chat(question string) string {
	if openaiClient == nil {
		openaiClient = openai.NewClient(option.WithBaseURL("http://192.168.101.130:8011/v1"), option.WithDebugLog(logger.Logger))
	}

	ctx := context.Background()

	l.Messages = append(l.Messages, openai.UserMessage(question))

	for len(l.Messages) > l.MessagesLenLimit {
		l.Messages = append(l.Messages[:0], l.Messages[2:]...) //保留系统提示词，删除最老的对话记录
	}

	params := openai.ChatCompletionNewParams{
		Messages: l.Messages,
		Tools: []openai.ChatCompletionToolUnionParam{
			openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name:        "get_weather",
				Description: openai.String("Get weather at the given location"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]string{
							"type": "string",
						},
					},
					"required": []string{"location"},
				},
			}),
		},
		Seed:  openai.Int(0),
		Model: "Qwen3-8B-AWQ",
	}

	// Make initial chat completion request
	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		panic(err)
	}

	toolCalls := completion.Choices[0].Message.ToolCalls

	// Return early if there are no tool calls
	// if len(toolCalls) == 0 {
	// 	fmt.Printf("No function call")
	// 	return
	// }

	// If there is a was a function call, continue the conversation
	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "get_weather" {
			// Extract the location from the function call arguments
			var args map[string]interface{}
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				panic(err)
			}
			location := args["location"].(string)

			// Simulate getting weather data
			weatherData := getWeather(location)

			// Print the weather data
			fmt.Printf("Weather in %s: %s\n", location, weatherData)

			params.Messages = append(params.Messages, openai.ToolMessage(weatherData, toolCall.ID))
		}
	}

	completion, err = client.Chat.Completions.New(ctx, params)
	if err != nil {
		panic(err)
	}
	println(completion.Choices[0].Message.Content)
}
