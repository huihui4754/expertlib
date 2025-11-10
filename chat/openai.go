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
	DialogID               string                                   `json:"dialog_id"`
	Messages               []openai.ChatCompletionMessageParamUnion `json:"messages"` // 不包括系统提示词
	AIURL                  string                                   `json:"-"`
	AIModel                string                                   `json:"-"`
	ExpertChatSystemPrompt string                                   `json:"-"` // 专家返回必须的内置系统提示词
	SystemPrompt           string                                   `json:"-"` //用户设置的个性化系统提示词
	MessagesLenLimit       int                                      `json:"-"`
	LastSavedContentMd5    string                                   `json:"-"`
	callFuctionCall        func(call *FunctionCall) (string, error) `json:"-"`
	Relpying               bool                                     `json:"-"`
}

func (l *OpenaiChatLLM) deleteOldMessage() {
	if len(l.Messages) > l.MessagesLenLimit {
		for len(l.Messages) > l.MessagesLenLimit {
			l.Messages = l.Messages[len(l.Messages)-l.MessagesLenLimit:]
		}
	}
}

// 第一轮使用用户设置的提示词查找是否需要使用工具，如果需要就调用，并将结果传给大模型并带上专家的系统提示词
func (l *OpenaiChatLLM) Chat(question string, tools []openai.ChatCompletionToolUnionParam) (string, error) {
	initOnce.Do(func() {
		openaiClient = openai.NewClient(option.WithBaseURL(l.AIURL))
	})

	if l.Relpying {
		return "当前dialog llm 还未回复完", errors.New("当前dialog llm 还未回复完")
	}

	ctx := context.Background()

	l.Messages = append(l.Messages, openai.UserMessage(question))
	l.deleteOldMessage()

	messageWithOutExpertSystem := make([]openai.ChatCompletionMessageParamUnion, 0, messagesLenLimit+2)
	messageWithOutExpertSystem = append(messageWithOutExpertSystem, openai.SystemMessage(l.SystemPrompt))
	messageWithOutExpertSystem = append(messageWithOutExpertSystem, l.Messages...)

	paramsWithoutExpertSystem := openai.ChatCompletionNewParams{
		Messages: messageWithOutExpertSystem,
		Tools:    tools,
		// Seed:     openai.Int(0),
		Model: l.AIModel,
	}

	completion1, err := openaiClient.Chat.Completions.New(ctx, paramsWithoutExpertSystem)
	if err != nil {
		logger.Errorf("chat with openaiClient err: %v", err)
		return "请求大模型失败", err
	}

	paramsWithoutExpertSystemData, err := json.Marshal(paramsWithoutExpertSystem)
	if err == nil {
		logger.Debugf("paramsWithoutExpertSystemData 1: %v", string(paramsWithoutExpertSystemData))
	}

	data1, err := json.Marshal(completion1)
	if err == nil {
		logger.Debugf("completion 1: %v", string(data1))
	}

	toolCalls1 := completion1.Choices[0].Message.ToolCalls

	logger.Debugf("toolCalls1 len : %v", len(toolCalls1))

	messageWithExpertSystem := make([]openai.ChatCompletionMessageParamUnion, 0, messagesLenLimit+2)
	messageWithExpertSystem = append(messageWithExpertSystem, openai.SystemMessage(l.ExpertChatSystemPrompt))
	messageWithExpertSystem = append(messageWithExpertSystem, l.Messages...)

	if len(toolCalls1) == 0 {
		logger.Debug("no need call tool ")
	} else {
		paramsWithoutExpertSystem.Messages = append(paramsWithoutExpertSystem.Messages, completion1.Choices[0].Message.ToParam())
		for _, toolCall := range toolCalls1 {
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
				paramsWithoutExpertSystem.Messages = append(paramsWithoutExpertSystem.Messages, openai.ToolMessage(result, toolCall.ID))
			}
		}

		completion2, err := openaiClient.Chat.Completions.New(ctx, paramsWithoutExpertSystem)
		if err != nil {
			logger.Errorf("chat with openaiClient err: %v", err)
			return "请求大模型失败", err
		}
		logger.Debugf("工具初步调用结果 ： %v", completion2.Choices[0].Message.Content)
		messageWithExpertSystem = append(messageWithExpertSystem, openai.AssistantMessage(completion2.Choices[0].Message.Content))
		// l.Messages = append(l.Messages, openai.AssistantMessage(completion2.Choices[0].Message.Content))

	}

	logger.Debugf("tool len : %v", len(tools))

	params := openai.ChatCompletionNewParams{
		Messages: messageWithExpertSystem,
		Tools:    tools,
		// Seed:     openai.Int(0),
		Model: l.AIModel,
	}
	data, err := json.Marshal(params)
	if err == nil {
		logger.Debugf("requset parm : %v", string(data))
	}
	// Make initial chat completion request
	completion, err := openaiClient.Chat.Completions.New(ctx, params)
	if err != nil {
		logger.Errorf("chat with openaiClient err: %v", err)
		return "请求大模型失败", err
	}

	data, err = json.Marshal(completion)
	if err == nil {
		logger.Debugf("completion : %v", string(data))
	}

	l.Messages = append(l.Messages, openai.AssistantMessage(completion.Choices[0].Message.Content))
	l.deleteOldMessage()
	return completion.Choices[0].Message.Content, nil
}

func (l *OpenaiChatLLM) SetCallFuncHandler(callFuncHandler func(call *FunctionCall) (string, error)) {
	l.callFuctionCall = callFuncHandler
}
