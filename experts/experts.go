package experts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/loglevel"
	"github.com/huihui4754/expertlib/types"
)

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

func SetLogger(level int) {
	logger.SetLevel(level)
}

type TotalMessage = types.TotalMessage
type DialogInfo = types.DialogInfo

// Expert结构体保存expert实例的配置和处理程序。
type Expert struct {
	dataFilePath        string
	rnnIntentPath       string
	onnxLibPath         string
	commandFirst        bool
	userMessageHandler  func(any, string)
	toolMessageHandler  func(any, string)
	chatMessageHandler  func(any, string)
	intentMatch         *IntentMatchManager //意图识别管理器
	rnnIntent           *RNNIntentManager   //RNN意图管理器
	messageInChan       chan TotalMessage
	dialogs             map[string]*DialogInfo
	dialogsMutex        *sync.RWMutex
	lastSavedDialogInfo string // 上次保存的dialog 信息的字符串表示
	saveDialogInfoFunc  func(map[string]*DialogInfo)
	loadDialogInfoFunc  func() map[string]*DialogInfo
	saveInterval        time.Duration // 定时保存dialog 和 意图识别间隔时间
}

// NewExpert会建立Expert的新执行严修。
func NewExpert() *Expert {
	intentsManager := NewIntentManager()
	return &Expert{
		intentMatch:   intentsManager,
		rnnIntentPath: "",
		onnxLibPath:   "",
		messageInChan: make(chan TotalMessage, 1000),
		dialogs:       make(map[string]*DialogInfo),
		dialogsMutex:  &sync.RWMutex{},
	}
}

func (t *Expert) SetMessageFormatFunc(formatting func(string) string) {
	t.intentMatch.SetMessageFormatFunc(formatting)
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

func (t *Expert) SetONNXLibPath(path string) {

}

func (t *Expert) SetSaveIntervalTime(interval time.Duration) {
	t.saveInterval = interval
	logger.Info("Save interval time set to:", interval)
}

func (t *Expert) defaultDialogPath() string {
	return filepath.Join(t.dataFilePath, "user", "dailoginfo.json")
}

// 设置保存dialog信息的处理函数，可以自定义保存逻辑，不会保存到默认文件路径，设置后会定时触发保存
func (t *Expert) SetSaveDialogInfoHandler(handler func(map[string]*DialogInfo)) {
	t.saveDialogInfoFunc = handler
}

// 设置加载dialog信息的处理函数，可以自定义加载逻辑，不会从默认文件路径加载，在 run 启动设置后会调用此函数加载
func (t *Expert) SetLoadDialogInfoHandler(handler func() map[string]*DialogInfo) {
	t.loadDialogInfoFunc = handler
}

// loadUserInfo 加载dialog信息到内存。
func (t *Expert) loadDialogInfo() {
	t.dialogsMutex.Lock()
	defer t.dialogsMutex.Unlock()

	if t.loadDialogInfoFunc != nil {
		t.dialogs = t.loadDialogInfoFunc()
		logger.Infof("Loaded %d user info records from custom loader.", len(t.dialogs))
		return
	}

	if t.dataFilePath != "" {
		data, err := os.ReadFile(t.defaultDialogPath())
		if err != nil {
			if os.IsNotExist(err) {
				logger.Info("User info file not found, starting with empty data.")
				return
			}
			logger.Errorf("Failed to read user info file: %v", err)
			return
		}

		if err := json.Unmarshal(data, &t.dialogs); err != nil {
			logger.Errorf("Failed to unmarshal user info data: %v", err)
			return
		}

		// Store the initial state of the data
		initialData, err := json.MarshalIndent(t.dialogs, "", "  ")
		if err == nil {
			t.lastSavedDialogInfo = string(initialData)
		}

		logger.Infof("Loaded %d user info records.", len(t.dialogs))
	}

}

// 保存daialog信息
func (t *Expert) saveDialogInfo() {
	t.dialogsMutex.Lock()
	defer t.dialogsMutex.Unlock()

	if t.saveDialogInfoFunc != nil {
		t.saveDialogInfoFunc(t.dialogs)
		logger.Info("Saved user info using custom saver.")
		return
	}

	data, err := json.MarshalIndent(t.dialogs, "", "  ")
	if err != nil {
		logger.Errorf("Failed to marshal user info data: %v", err)
		return
	}

	if string(data) == t.lastSavedDialogInfo {
		return // No changes to save
	}

	if err := os.WriteFile(t.defaultDialogPath(), data, 0644); err != nil {
		logger.Errorf("Failed to write user info file: %v", err)
	} else {
		logger.Info("Save  user info successfully.")
		t.lastSavedDialogInfo = string(data)
	}
}

func (t *Expert) periodicSave() {
	ticker := time.NewTicker(t.saveInterval)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug("Periodic save check")
		t.saveDialogInfo()
	}
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
		logger.Info("Loading intent cache from path:", t.dataFilePath)
		t.intentMatch.SetCacheFilePath(t.dataFilePath)
		t.intentMatch.LoadIntentCache()
		go t.intentMatch.PeriodicCacheSave(t.saveInterval)
	}

	t.loadDialogInfo()
	go t.periodicSave()

	for chunk := range t.messageInChan {
		logger.Debug("Processing message from channel:", chunk)
	}
	// 启动逻辑的占位符
}

// GetAllIntentNames returns all intent names.
func (t *Expert) GetAllIntentNames() []string {
	logger.Debug("Getting all intent names")
	matchers := t.intentMatch.GetALLNewIntentMatcher()
	names := make([]string, 0, len(matchers))
	for _, m := range matchers {
		if m == nil {
			names = append(names, "")
			continue
		}
		names = append(names, m.GetIntentName())
	}
	return names
}

// UpdateIntentMatcher 从设置的路径更新意图匹配器。
func (t *Expert) UpdateIntentMatcher() {
	logger.Info("Updating intent matcher...")
	// 实际逻辑的占位符
	//todo 此处重新加载所有意图匹配器
}
