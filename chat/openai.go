package chat

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/huihui4754/expertlib/types"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

var (
	openaiClient openai.Client
	initOnce     sync.Once
)

type ChatAIMessage = types.ChatAIMessage

type OpenaiChatLLM struct {
	DialogID            string                                   `json:"dialog_id"`
	Messages            []openai.ChatCompletionMessageParamUnion `json:"messages"` // 不包括系统提示词
	AIURL               string                                   `json:"-"`
	AIModel             string                                   `json:"-"`
	SystemPrompt        string                                   `json:"-"`
	MessagesLenLimit    int                                      `json:"-"`
	LastSavedContentMd5 string                                   `json:"-"`
	callFuctionCall     func(call *FunctionCall) (string, error) `json:"-"`
	Relpying            bool                                     `json:"-"`
}

func (l *OpenaiChatLLM) deleteOldMessage() {
	if len(l.Messages) > l.MessagesLenLimit {
		for len(l.Messages) > l.MessagesLenLimit {
			l.Messages = l.Messages[len(l.Messages)-l.MessagesLenLimit:]
		}
	}
}

func (l *OpenaiChatLLM) Chat(question string, tools []openai.ChatCompletionToolUnionParam) (string, error) {
	initOnce.Do(func() {
		openaiClient = openai.NewClient(option.WithBaseURL(l.AIURL))
	})

	if l.Relpying {
		return "当前dialog llm 还未回复完", errors.New("当前dialog llm 还未回复完")
	}

	ctx := context.Background()

	l.Messages = append(l.Messages, openai.UserMessage(question))

	logger.Debugf("messages :%v", l.Messages)
	l.deleteOldMessage()
	logger.Debugf("messages :%v", l.Messages)

	messageWithSystem := make([]openai.ChatCompletionMessageParamUnion, 0, messagesLenLimit+2)
	messageWithSystem = append(messageWithSystem, openai.SystemMessage(l.SystemPrompt))

	messageWithSystem = append(messageWithSystem, l.Messages...)

	params := openai.ChatCompletionNewParams{
		Messages: messageWithSystem,
		Tools:    tools,
		// Seed:     openai.Int(0),
		Model: l.AIModel,
	}

	// Make initial chat completion request
	completion, err := openaiClient.Chat.Completions.New(ctx, params)
	if err != nil {
		logger.Errorf("chat with openaiClient err: %v", err)
		return "请求大模型失败", err
	}

	toolCalls := completion.Choices[0].Message.ToolCalls

	if len(toolCalls) == 0 {
		l.Messages = append(l.Messages, openai.AssistantMessage(completion.Choices[0].Message.Content))
		for len(l.Messages) > l.MessagesLenLimit {
			l.Messages = l.Messages[len(l.Messages)-l.MessagesLenLimit:]
		}
		return completion.Choices[0].Message.Content, nil
	}

	// If there is a was a function call, continue the conversation
	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	for _, toolCall := range toolCalls {
		var args map[string]interface{}
		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		if err != nil {
			logger.Errorf("get tool arguments err: %v", err)
			continue
		}
		if l.callFuctionCall != nil {
			result, err := l.callFuctionCall(&FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: args,
			})
			if err != nil {
				logger.Errorf("call tool err: %v", err)
				continue
			}
			params.Messages = append(params.Messages, openai.ToolMessage(result, toolCall.ID))
		}
	}

	completion, err = openaiClient.Chat.Completions.New(ctx, params)
	if err != nil {
		logger.Errorf("chat with openaiClient err: %v", err)
		return "请求大模型失败", err
	}
	l.Messages = append(l.Messages, openai.AssistantMessage(completion.Choices[0].Message.Content))
	for len(l.Messages) > l.MessagesLenLimit {
		l.Messages = l.Messages[len(l.Messages)-l.MessagesLenLimit:]
	}
	return completion.Choices[0].Message.Content, nil
}

func (l *OpenaiChatLLM) SetCallFuncHandler(callFuncHandler func(call *FunctionCall) (string, error)) {
	l.callFuctionCall = callFuncHandler
}
