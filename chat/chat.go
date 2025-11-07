package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
	"github.com/openai/openai-go/v3"
)

type TotalMessage = types.TotalMessage
type Attachment = types.Attachment

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
	systemPrompt           string
	llmChatManager         LLMChatWithFunCallManager
	expertMessageHandler   func(TotalMessage, string)
	expertMessageInChan    chan *TotalMessage //消息输入通道
	toExpertMessageOutChan chan *TotalMessage
	// FunctionCalls          []Funcall
}

type Funcall struct {
	// Define the structure of a function call
}

func NewChat() *Chat {
	defalutDataPath := ""
	currentUser, err := user.Current() // todo 后续支持从配置文件读取配置
	if err != nil {
		fmt.Printf("获取用户信息失败：%v\n", err)
	} else {
		defalutDataPath = filepath.Join(currentUser.HomeDir, "expert", "chat")
	}
	return &Chat{
		dataFilePath:           defalutDataPath,
		llmUrl:                 "",
		modelName:              "",
		systemPrompt:           "",
		expertMessageInChan:    make(chan *TotalMessage),
		toExpertMessageOutChan: make(chan *TotalMessage),
		llmChatManager: LLMChatWithFunCallManager{
			SaveIntervalTime: 20 * time.Minute,
			llmsMutex:        &sync.Mutex{},
		},
	}
}

func (c *Chat) SetDataFilePath(path string) {
	c.dataFilePath = path
	c.llmChatManager.DataPath = path
	logger.Info("dataFilePath set to:", path)
}

func (c *Chat) SetLLMUrl(url string) {
	c.llmUrl = url
	c.llmChatManager.AIURL = url
	logger.Info("llmUrl set to:", url)
}

func (c *Chat) SetModelName(model string) {
	c.modelName = model
	c.llmChatManager.AIModel = model
	logger.Info("AIModel set to:", model)
}

// 设置多轮对话的个性化提示词，不可设置回复内容格式，否者会无法回复。
func (c *Chat) SetSystemPrompt(prompt string) {
	c.systemPrompt = prompt
	c.llmChatManager.SystemPrompt = prompt
	logger.Info("SystemPrompt set to:", prompt)
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

func (c *Chat) SetCallFunctionHandler(callFuncHandler func(call *FunctionCall) (string, error)) {
	c.llmChatManager.SetCallFuncHandler(callFuncHandler)
}

func (c *Chat) SetToExpertMessageHandler(handler func(TotalMessage, string)) {
	c.expertMessageHandler = handler
	logger.Info("expertMessageHandler set")
}

func (c *Chat) Run() {

	// Start the chat instance here
	if c.llmUrl == "" || c.modelName == "" {
		panic("你必须在执行 Run 前设置 大模型链接和模型名称")
	}

	if c.dataFilePath != "" {
		go c.llmChatManager.PeriodicSave()
	}

	logger.Info("Chat instance running")

	for {
		select {
		case expertMsg := <-c.expertMessageInChan:
			go c.handleFromExpertMessage(expertMsg)
		case toExpertMsg := <-c.toExpertMessageOutChan:
			go func() {
				toExpertMessage := *toExpertMsg
				msg, err := json.Marshal(toExpertMessage)
				if err != nil {
					logger.Error("Failed to marshal chat message: %v", err)
				}
				c.expertMessageHandler(toExpertMessage, string(msg))
			}()
		}
	}

}

func (c *Chat) handleFromExpertMessage(message *TotalMessage) {

	switch message.EventType {
	case 1001:
		logger.Debug("专家发送消息")
		res := c.llmChatManager.ChatLLM(message)
		if res != nil {
			c.toExpertMessageOutChan <- res
		}

	case 1002:
		logger.Debug("专家终止对话")

	default:
		log.Printf("收到未知事件类型: %d", message.EventType)
	}

}

func (c *Chat) SetModelContextFunctionTools(callTools []ModelContextFunctionTool) {
	c.llmChatManager.SetTools(callTools)
	logger.Info("callTools set")
}

// 和上面 SetModelContextFunctionTools 二选一 使用即可
func (c *Chat) SetOpenaiChatCompletionToolUnionParam(openaiTool []openai.ChatCompletionToolUnionParam) {
	c.llmChatManager.SetOpenaiChatCompletionToolUnionParam(openaiTool)
	logger.Info("openaiTool set")
}
