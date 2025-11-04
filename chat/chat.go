package chat

import (
	"encoding/json"

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
	dataFilePath           string
	llmUrl                 string
	modelName              string
	requestLLMHeaders      string
	systemPrompt           string
	FunctionCalls          []Funcall
	expertMessageHandler   func(TotalMessage, string)
	expertMessageInChan    chan *TotalMessage //专家消息输入通道
	toExpertMessageOutChan chan *TotalMessage
}

type Funcall struct {
	// Define the structure of a function call
}

func NewChat() *Chat {
	return &Chat{}
}

func (c *Chat) SetDataFilePath(path string) {
	c.dataFilePath = path
	logger.Info("dataFilePath set to:", path)
}

func (c *Chat) SetLLMUrl(url string) {
	c.llmUrl = url
	logger.Info("llmUrl set to:", url)
}

func (c *Chat) SetModelName(model string) {
	c.modelName = model
}

func (c *Chat) SetRequestLLMHeaders(headers string) {
	c.requestLLMHeaders = headers
}

func (c *Chat) SetSystemPrompt(prompt string) {

}

func (c *Chat) SetFunctionCall(functions []Funcall) {
	c.FunctionCalls = functions
	logger.Info("FunctionCall set")
}

func (c *Chat) HandleExpertRequestMessage(message any) {
	var messagePointer *TotalMessage
	var err error
	switch v := message.(type) {
	case TotalMessage:
		// 复制值类型，取新地址
		msg := v
		messagePointer = &msg
	case *TotalMessage:
		if v == nil {
			logger.Error("*TotalMessage 为 nil")
			break
		}
		// 复制指针指向的值，取新地址（避免外部修改影响）
		msg := *v // 解引用并复制
		messagePointer = &msg
	case string:
		var totalMsg TotalMessage
		err = json.Unmarshal([]byte(v), &totalMsg)
		if err == nil {
			messagePointer = &totalMsg
		} else {
			logger.Errorf("无法解析字符串消息为 TotalMessage  message: %v,  err: %v", v, err)
		}
	case []byte:
		var totalMsg TotalMessage
		err = json.Unmarshal(v, &totalMsg)
		if err == nil {
			messagePointer = &totalMsg
		} else {
			logger.Errorf("无法解析bytes消息为 TotalMessage  bytes: %v,  err: %v", string(v), err)
		}

	default:
		logger.Error("不支持的消息结构")
	}
	if c.expertMessageInChan != nil && messagePointer != nil && err == nil {
		c.expertMessageInChan <- messagePointer
	}
}

func (c *Chat) SetCallFunctionHandler() {

}

func (c *Chat) SetToExpertMessageHandler(handler func(TotalMessage, string)) {
	c.expertMessageHandler = handler
	logger.Info("expertMessageHandler set")
}

func (c *Chat) Run() {
	// Start the chat instance here
	logger.Info("Chat instance running")

	for {
		select {
		case expertMsg := <-c.expertMessageInChan:
			go c.handleFromUserMessage(userMsg)
		case toExpertMsg := <-c.toExpertMessageOutChan:
			go c.handleFromProgramMessage(programMsg)
		}
	}

}
