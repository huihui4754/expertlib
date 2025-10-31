package chat

import (
	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
)

type TotalMessage = types.TotalMessage

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

func SetLogger(level loglevel.Level) {
	logger.SetLevel(level)
}

type Chat struct {
	DataFilePath         string
	LLMUrl               string
	SystemPrompt         string
	FunctionCalls        []Funcall
	ExpertMessageHandler func(TotalMessage, string)
}

type Funcall struct {
	// Define the structure of a function call
}

func NewChat() *Chat {
	return &Chat{}
}

func (t *Chat) SetDataFilePath(path string) {
	t.DataFilePath = path
	logger.Info("DataFilePath set to:", path)
}

func (t *Chat) SetLLMUrl(url string) {
	t.LLMUrl = url
	logger.Info("LLMUrl set to:", url)
}

func (t *Chat) SetSystemPrompt(prompt string) {
	t.SystemPrompt = prompt
	logger.Info("SystemPrompt set to:", prompt)
}

func (t *Chat) SetFunctionCall(functions []funcall) {
	t.FunctionCalls = functions
	logger.Info("FunctionCall set")
}

func (t *Chat) HandleExpertRequestMessage(jsonx any) {
	// Handle the Expert request message here
	logger.Debug("Handling Expert request message:", jsonx)
}

func (t *Chat) SetToExpertMessageHandler(handler func(TotalMessage, string)) {
	t.ExpertMessageHandler = handler
	logger.Info("ExpertMessageHandler set")
}

func (t *Chat) Run() {
	// Start the chat instance here
	logger.Info("Chat instance running")
}
