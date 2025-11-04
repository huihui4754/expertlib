package types

// 定义llm functioncall 类型
type FunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// 给大模型的可序列化对象
type LLMCallableTool struct {
	Type     string                   `json:"type" validate:"required"` //值固定为 function
	Function ModelContextFunctionTool `json:"function" validate:"required"`
}

// ModelContextFunctionTool 原始需要的function tool 对象
type ModelContextFunctionTool struct {
	Name        string      `json:"name"  validate:"required"`        // 函数或工具名称
	Description string      `json:"description"  validate:"required"` // 函数或工具功能描述
	Parameters  InputSchema `json:"parameters"  validate:"required"`  // 所需要的参数
}

type InputSchema struct {
	Properties map[string]Property `json:"properties"  validate:"required"` //所有参数说明
	Required   []string            `json:"required"  validate:"required"`   // 必须的参数名称
	Type       string              `json:"type"  validate:"required"`       // 值固定为 object
}

type Property struct {
	Type        string `json:"type"  validate:"required"`        // 参数的类型   值可能为 "object" "string" "number" "integer" "boolean" "array" "enum" "anyOf" 这些之一
	Description string `json:"description"  validate:"required"` // 参数的说明
}

type ToolCallStreamCache struct {
	Name      string `json:"name" empty:""`
	Arguments string `json:"arguments" empty:""`
}

type ChatAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
