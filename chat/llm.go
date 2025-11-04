package chat

import "github.com/huihui4754/expertlib/types"

type FunctionCall = types.FunctionCall
type LLMCallableTool = types.LLMCallableTool
type InputSchema = types.InputSchema
type ModelContextFunctionTool = types.ModelContextFunctionTool

type LLMChatWithFunCallInter interface {
	Chat(TotalMessage) TotalMessage //和用户聊天并返回的接口
	SetLLMCallableTool([]LLMCallableTool)
	SetCallFuncHandler(func(call *FunctionCall) (string, error))
}

type LLMChatWithFunCall struct {
	AIURL        string            // 大模型url
	AIModel      string            // 模型名称
	SystemPrompt string            // 系统提示词
	ExtraBody    map[string]any    //配置请求体中的参数
	ExtraHeader  map[string]string //配置头部中的参数
}

// for _, v := range h.OriginalMcpTools {
// 		h.LLMCallableTools = append(h.LLMCallableTools, LLMCallableTool{
// 			Type:     "function",
// 			Function: v,
// 		})
// 	}
