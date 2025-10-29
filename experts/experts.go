package experts

import (
	"time"

	"github.com/huihui4754/expertlib/loglevel"
)

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

func SetLogger(level int) {
	logger.SetLevel(level)
}

// Expert结构体保存expert实例的配置和处理程序。
type Expert struct {
	dataFilePath       string
	rnnIntentPath      string
	onnxLibPath        string
	commandFirst       bool
	userMessageHandler func(any, string)
	toolMessageHandler func(any, string)
	chatMessageHandler func(any, string)
	rnnIntent          *RNNIntentManager   //RNN意图识别管理器
	intentMatch        *IntentMatchManager //意图识别管理器
}

// NewExpert会建立Expert的新执行严修。
func NewExpert() *Expert {
	intentsManager := NewIntentManager()
	return &Expert{
		intentMatch:   intentsManager,
		rnnIntentPath: "",
		onnxLibPath:   "",
	}
}

func (t *Expert) Register(intentMatcher func() IntentMatchInter, intentName string) {
	t.intentMatch.Register(intentMatcher, intentName)
}

func (t *Expert) UnRegister(intentName string) {
	t.intentMatch.UnRegister(intentName)
}

// SetDataFilePath设置专家的数据文件路径。
func (t *Expert) SetDataFilePath(path string) {
	t.dataFilePath = path
	logger.Info("Data file path set to:", path)
}

// SetRNNIntentPath设置RNN Intent模型路径。
func (t *Expert) SetRNNIntentPath(path string) {
	t.rnnIntentPath = path
	logger.Info("RNN intent path set to:", path)
}

// SetCommandFirst设置在多轮对话中命令是否优先。
func (t *Expert) SetCommandFirst(commandFirst bool) {
	t.commandFirst = commandFirst
	logger.Info("Command first set to:", commandFirst)
}

// HandleUserRequestMessage 用户传给专家的消息由此进入
func (t *Expert) HandleUserRequestMessage(message any) {
	logger.Debug("HandleUserRequestMessage received:", message)
	if t.userMessageHandler != nil {
		// Placeholder for actual logic
		t.userMessageHandler("response from expert for user", "")
	}
}

// HandleUserRequestMessageString 用户传给专家的消息由此进入 ，一条消息只用传输一次，和上面二选一
func (t *Expert) HandleUserRequestMessageString(message string) {
	logger.Debug("HandleUserRequestMessageString received:", message)
	if t.userMessageHandler != nil {
		// Placeholder for actual logic
		t.userMessageHandler("response from expert for user", "")
	}
}

// SetUserMessage 设置返回给用户的消息处理函数
func (t *Expert) SetUserMessageHandler(handler func(any, string)) {
	t.userMessageHandler = handler
}

// HandleToolRequestMessage  程序库（工具）传给专家的消息由此进入
func (t *Expert) HandleToolRequestMessage(message any) {
	logger.Debug("HandleToolRequestMessage received:", message)
	if t.toolMessageHandler != nil {
		// 实际逻辑的占位符
		t.toolMessageHandler("response from expert for tool", "")
	}
}

// HandleToolRequestMessageString  程序库（工具）传给专家的消息由此进入 ，一条消息只用传输一次，和上面二选一
func (t *Expert) HandleToolRequestMessageString(message string) {
	logger.Debug("HandleToolRequestMessageString received:", message)
	if t.toolMessageHandler != nil {
		// 实际逻辑的占位符
		t.toolMessageHandler("response from expert for tool", "")
	}
}

// SetUserMessage 设置返回给工具的消息处理函数
func (t *Expert) SetToToolMessageHandler(handler func(any, string)) {
	t.toolMessageHandler = handler
}

// HandleChatRequestMessage  多轮对话传给专家的消息由此进入
func (t *Expert) HandleChatRequestMessage(message any) {
	logger.Debug("HandleChatRequestMessage received:", message)
	if t.chatMessageHandler != nil {
		// 实际逻辑的占位符
		t.chatMessageHandler("response from expert for chat", "")
	}
}

// HandleChatRequestMessageString 多轮对话传给专家的消息由此进入 ，一条消息只用传输一次，和上面二选一
func (t *Expert) HandleChatRequestMessageString(message string) {
	logger.Debug("HandleChatRequestMessageString received:", message)
	if t.chatMessageHandler != nil {
		// 实际逻辑的占位符
		t.chatMessageHandler("response from expert for chat", "")
	}
}

// SetToChatMessageHandler 设置返回给多轮对话的消息处理函数
func (t *Expert) SetToChatMessageHandler(handler func(any, string)) {
	t.chatMessageHandler = handler
}

// 前台占用启动专家实例。
func (t *Expert) Run() {
	logger.Info("Expert is running...")
	if t.rnnIntentPath != "" || t.onnxLibPath != "" {
		// 加载所有rnn 意图识别并注册到意图匹配管理器
		logger.Info("Initializing RNN Intent Manager...")
		rnnManager := NewRNNIntentManager()
		rnnManager.SetRNNModelPath(t.rnnIntentPath)
		rnnManager.SetLibPath(t.onnxLibPath)
		rnnManager.LoadRNNModelIntents()

		for name, intent := range rnnManager.GetAllRNNIntents() {
			t.intentMatch.Register(intent.GetIntentExpertMatch, name)
		}
		t.rnnIntent = rnnManager
		logger.Info("RNN Intent Manager initialized and intents registered.")
	}
	if t.dataFilePath != "" {
		// 如果设置了数据文件路径，则加载意图缓存，并启动定期保存
		logger.Info("Loading intent cache from file:", t.dataFilePath)
		t.intentMatch.SetCacheFilePath(t.dataFilePath)
		t.intentMatch.LoadIntentCache()
		go t.intentMatch.PeriodicCacheSave(10 * time.Minute)
	}

	for {

	}
	// 启动逻辑的占位符
}

// GetAllIntentNames returns all intent names.
func (t *Expert) GetAllIntentNames() []string {
	logger.Debug("Getting all intent names")
	// 实际逻辑的占位符
	return []string{"intent1", "intent2"}
}

// UpdateIntentMatcher 从设置的路径更新意图匹配器。
func (t *Expert) UpdateIntentMatcher() {
	logger.Info("Updating intent matcher...")
	// 实际逻辑的占位符
}
