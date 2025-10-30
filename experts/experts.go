package experts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
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
	dataFilePath          string
	rnnIntentPath         string
	onnxLibPath           string
	commandFirst          bool
	userMessageHandler    func(any, string)
	programMessageHandler func(any, string)
	chatMessageHandler    func(any, string)
	intentMatch           *IntentMatchManager //意图识别管理器
	rnnIntent             *RNNIntentManager   //RNN意图管理器
	UserMessageInChan     chan *TotalMessage  //用户消息输入通道
	ProgramMessageInChan  chan *TotalMessage  //程序库消息输入通道
	ChatMessageInChan     chan *TotalMessage  //多轮对话消息输入通道
	dialogs               map[string]*DialogInfo
	dialogsMutex          *sync.RWMutex
	lastSavedDialogInfo   string // 上次保存的dialog 信息的字符串表示
	saveDialogInfoFunc    func(map[string]*DialogInfo)
	loadDialogInfoFunc    func() map[string]*DialogInfo
	saveInterval          time.Duration // 定时保存dialog 和 意图识别间隔时间
}

// NewExpert会建立Expert的新执行严修。
func NewExpert() *Expert {
	intentsManager := NewIntentManager()
	defalutRnnModelPath := ""
	defalutDataPath := ""
	currentUser, err := user.Current()
	if err != nil {
		fmt.Printf("获取用户信息失败：%v\n", err)
	} else {
		defalutRnnModelPath = filepath.Join(currentUser.HomeDir, "expert", "rnnmodel")
		defalutDataPath = filepath.Join(currentUser.HomeDir, "expert", "dialog")
	}
	return &Expert{
		intentMatch:          intentsManager,
		rnnIntentPath:        defalutRnnModelPath,
		dataFilePath:         defalutDataPath,
		onnxLibPath:          "",
		UserMessageInChan:    make(chan *TotalMessage, 1000),
		ProgramMessageInChan: make(chan *TotalMessage, 1000),
		ChatMessageInChan:    make(chan *TotalMessage, 1000),
		saveInterval:         10 * time.Minute,
		dialogs:              make(map[string]*DialogInfo),
		dialogsMutex:         &sync.RWMutex{},
	}
}

// SetMessageFormatFunc 设置消息处理后再送入意图识别器的格式化函数  ，避免消息中部分数据和训练数据无关影响识别等
func (t *Expert) SetMessageFormatFunc(formatting func(string) string) {
	t.intentMatch.SetMessageFormatFunc(formatting)
}

// 可以通过此接口来注册意图匹配器
func (t *Expert) Register(intentMatcher func() IntentMatchInter, intentName string) {
	t.intentMatch.Register(intentMatcher, intentName)
}

// UnRegister 注销某个意图匹配器
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

// 设置ONNX动态库文件路径，用于RNN意图识别。需要提前下载放置好
func (t *Expert) SetONNXLibPath(path string) {
	t.onnxLibPath = path
	logger.Info("ONNX library path set to:", path)
}

// SetSaveIntervalTime 设置保存dialog信息和意图保存的时间间隔
func (t *Expert) SetSaveIntervalTime(interval time.Duration) {
	t.saveInterval = interval
	logger.Info("Save interval time set to:", interval)
}

// 内部使用，获取默认保存dialog信息的路径
func (t *Expert) defaultDialogPath() string {
	return filepath.Join(t.dataFilePath, "user", "dailoginfo.json")
}

// 内部使用，获取默认保存意图匹配缓存的路径
func (t *Expert) defaultIntentMatchCachePath() string {
	return filepath.Join(t.dataFilePath, "user", "intentMatchCache.json")
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

// 定期保存dialog信息
func (t *Expert) periodicSave() {
	ticker := time.NewTicker(t.saveInterval)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug("Periodic save check")
		t.saveDialogInfo()
	}
}

// HandleUserRequestMessage 用户传给专家的消息由此进入，可以传入多种格式。
func (t *Expert) HandleUserRequestMessage(message any) {
	logger.Debug("HandleUserRequestMessage received:", message)
	var messagePointer *TotalMessage //todo 后面可以优化成启动前初始化很多个 TotalMessage 指针，避免频繁分配内存,需要根据并发量决定是否使用，低频环境可能现在更适用
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
	if t.UserMessageInChan != nil && messagePointer != nil && err == nil {
		t.UserMessageInChan <- messagePointer
	}
}

// SetUserMessage 设置返回给用户的消息处理函数
func (t *Expert) SetUserMessageHandler(handler func(any, string)) {
	t.userMessageHandler = handler
}

// HandleProgramRequestMessage  程序库（工具）传给专家的消息由此进入
func (t *Expert) HandleProgramRequestMessage(message any) {
	logger.Debug("HandleProgramRequestMessage received:", message)
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
	if t.ProgramMessageInChan != nil && messagePointer != nil && err == nil {
		t.ProgramMessageInChan <- messagePointer
	}
}

// SetUserMessage 设置返回给工具的消息处理函数
func (t *Expert) SetToProgramMessageHandler(handler func(any, string)) {
	t.programMessageHandler = handler
}

// HandleChatRequestMessage  多轮对话传给专家的消息由此进入
func (t *Expert) HandleChatRequestMessage(message any) {
	logger.Debug("HandleChatRequestMessage received:", message)
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
	if t.ChatMessageInChan != nil && messagePointer != nil && err == nil {
		t.ChatMessageInChan <- messagePointer
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
		logger.Info("Loading intent cache from path:", t.defaultIntentMatchCachePath())
		t.intentMatch.SetCacheFilePath(t.defaultIntentMatchCachePath())
		t.intentMatch.LoadIntentCache()
		go t.intentMatch.PeriodicCacheSave(t.saveInterval)
	}

	t.loadDialogInfo()
	go t.periodicSave()

	for chunk := range t.UserMessageInChan {
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
