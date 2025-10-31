package types

// 定义llm functioncall 类型
type FunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type InputSchema struct {
	Properties map[string]any `json:"properties"  validate:"required"`
	Required   []string       `json:"required"  validate:"required"`
	Type       string         `json:"type"  validate:"required"`
}

// ModelContextFunctionTool 原始需要的function tool 对象
type ModelContextFunctionTool struct {
	Name        string      `json:"name"  validate:"required"`
	Description string      `json:"description"  validate:"required"`
	InputSchema InputSchema `json:"inputSchema"  validate:"required"`
}

type LLMCallableTool struct {
	Type     string                   `json:"name" validate:"required"`
	Function ModelContextFunctionTool `json:"function" validate:"required"`
}
