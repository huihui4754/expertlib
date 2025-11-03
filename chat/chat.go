package chat

import (
	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
)

type TotalMessage = types.TotalMessage

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

func SetLogger(level loglevel.Level) {
	logger.SetLevel(level)
}

type Chat struct {
	dataFilePath         string
	llmUrl               string
	modelName            string
	requestLLMHeaders    string
	systemPrompt         string
	FunctionCalls        []Funcall
	expertMessageHandler func(TotalMessage, string)
	llm                  *openai.LLM
	messages             []schema.ChatMessage
}

type Funcall struct {
	// Define the structure of a function call
}

func NewChat() *Chat {
	return &Chat{}
}

func (t *Chat) SetDataFilePath(path string) {
	t.dataFilePath = path
	logger.Info("dataFilePath set to:", path)
}

func (t *Chat) SetLLMUrl(url string) {
	t.llmUrl = url
	logger.Info("llmUrl set to:", url)
}

func (t *Chat) SetModelName(model string) {
	t.modelName = model
}

func (t *Chat) SetRequestLLMHeaders(headers string) {
	t.requestLLMHeaders = headers
}

func (t *Chat) SetSystemPrompt(prompt string) {

}

func (t *Chat) SetFunctionCall(functions []Funcall) {
	t.FunctionCalls = functions
	logger.Info("FunctionCall set")
}

func (t *Chat) HandleExpertRequestMessage(jsonx any) {

}

func (t *Chat) SetCallFunctionHandler() {

}

func (t *Chat) SetToExpertMessageHandler(handler func(TotalMessage, string)) {
	t.expertMessageHandler = handler
	logger.Info("expertMessageHandler set")
}

func (t *Chat) Run() {
	// Start the chat instance here
	logger.Info("Chat instance running")

}
