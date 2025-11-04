package experts

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
)

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

func SetLogger(level loglevel.Level) {
	logger.SetLevel(level)
}

type TotalMessage = types.TotalMessage
type DialogInfo = types.DialogInfo

// Expert结构体保存expert实例的配置和处理程序。
type Expert struct {
	dataFilePath           string
	rnnIntentPath          string
	onnxLibPath            string
	commandFirst           bool
	userMessageHandler     func(TotalMessage, string)
	programMessageHandler  func(TotalMessage, string)
	chatMessageHandler     func(TotalMessage, string)
	intentMatch            *IntentMatchManager //意图识别管理器
	rnnIntent              *RNNIntentManager   //RNN意图管理器
	UserMessageInChan      chan *TotalMessage  //用户消息输入通道
	ProgramMessageInChan   chan *TotalMessage  //程序库消息输入通道
	ChatMessageInChan      chan *TotalMessage  //多轮对话消息输入通道
	dialogs                map[string]*DialogInfo
	dialogsMutex           *sync.RWMutex
	lastSavedDialogInfoMd5 string // 上次保存的dialog 信息的md5 值
	saveDialogInfoFunc     func(map[string]*DialogInfo)
	loadDialogInfoFunc     func() map[string]*DialogInfo
	saveInterval           time.Duration // 定时保存dialog 和 意图识别间隔时间
	chatSaveHistoryLimit   int           // 多轮对话保存的历史消息条数限制
}

// NewExpert会建立Expert的对象
func NewExpert() *Expert {
	intentsManager := NewIntentManager()
	defalutRnnModelPath := ""
	defalutDataPath := ""
	currentUser, err := user.Current() // todo 后续支持从配置文件读取配置
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
		chatSaveHistoryLimit: 20,
	}
}

// SetChatSaveHistoryLimit 设置多轮对话保存的历史消息条数限制
func (t *Expert) SetChatSaveHistoryLimit(limit int) {
	if limit <= 0 {
		limit = 1
	}
	t.chatSaveHistoryLimit = limit
	logger.Info("Chat save history limit set to:", limit)
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
			hash := md5.Sum(initialData)
			t.lastSavedDialogInfoMd5 = hex.EncodeToString(hash[:])
		}

		logger.Infof("Loaded %d user info records.", len(t.dialogs))
	}

}

// 保存daialog信息
func (t *Expert) saveDialogInfo() {

	if t.saveDialogInfoFunc != nil {
		t.dialogsMutex.Lock()
		t.saveDialogInfoFunc(t.dialogs)
		t.dialogsMutex.Unlock()
		logger.Info("Saved user info using custom saver.")
		return
	}
	t.dialogsMutex.Lock()
	data, err := json.MarshalIndent(t.dialogs, "", "  ")
	t.dialogsMutex.Unlock()
	if err != nil {
		logger.Errorf("Failed to marshal user info data: %v", err)
		return
	}

	hash := md5.Sum(data)
	currentHash := hex.EncodeToString(hash[:])

	if currentHash == t.lastSavedDialogInfoMd5 {
		return // No changes to save
	}

	if err := os.WriteFile(t.defaultDialogPath(), data, 0644); err != nil {
		logger.Errorf("Failed to write user info file: %v", err)
	} else {
		logger.Info("Save  user info successfully.")
		t.lastSavedDialogInfoMd5 = currentHash
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
func (t *Expert) SetToUserMessageHandler(handler func(TotalMessage, string)) {
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
func (t *Expert) SetToProgramMessageHandler(handler func(TotalMessage, string)) {
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
func (t *Expert) SetToChatMessageHandler(handler func(TotalMessage, string)) {
	t.chatMessageHandler = handler
}

func (t *Expert) getRNNIntentMangerFromFile() {
	if t.rnnIntentPath != "" || t.onnxLibPath != "" { // 目前只支持从文件系统中加载指定的rnn 意图识别器，后续可拓展，可以使用接口方便的在代码中加载意图识别器
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
}

// 前台占用启动专家实例。
func (t *Expert) Run() {
	logger.Info("Expert is running...")
	t.getRNNIntentMangerFromFile()
	if t.dataFilePath != "" {
		// 如果设置了数据文件路径，则加载意图缓存，并启动定期保存
		logger.Info("Loading intent cache from path:", t.defaultIntentMatchCachePath())
		t.intentMatch.SetCacheFilePath(t.defaultIntentMatchCachePath())
		t.intentMatch.LoadIntentCache()
		go t.intentMatch.PeriodicCacheSave(t.saveInterval)
	}

	t.loadDialogInfo()
	go t.periodicSave()

	for {
		select {
		case userMsg := <-t.UserMessageInChan:
			go t.handleFromUserMessage(userMsg)
		case programMsg := <-t.ProgramMessageInChan:
			go t.handleFromProgramMessage(programMsg)
		case chatMsg := <-t.ChatMessageInChan:
			go t.handleFromChatMessage(chatMsg)
		}
	}
	// 启动逻辑的占位符
}

// type ExpertToChatMessage = types.ExpertToChatMessage
// type ExpertToProgramMessage = types.ExpertToProgramMessage

func (t *Expert) handleFromUserMessage(message *TotalMessage) {
	dialogx, exists := t.dialogs[message.DialogID]
	if !exists {
		dialogx = &DialogInfo{
			UserID:      message.UserId,
			DialogID:    message.DialogID,
			Program:     "",
			ChatHistory: make([]string, 0),
		}
		t.dialogsMutex.Lock()
		t.dialogs[message.DialogID] = dialogx
		t.dialogsMutex.Unlock()
	}
	dialogx.RWMutex.Lock()
	defer dialogx.RWMutex.Unlock()
	switch message.EventType {
	case 1001: // 客户端发送消息

		logger.Infof("【用户提问】:%s", message.Messages.Content)

		// 如果从程序库返回消息 2003 不支持，且原封不动返回给专家，则不加入历史记录，避免重复
		historyLen := len(dialogx.ChatHistory)
		if historyLen > 0 && dialogx.ChatHistory[historyLen-1] == "User: "+message.Messages.Content {
			// 重复消息，不添加到历史记录
		} else {
			dialogx.ChatHistory = append(dialogx.ChatHistory, "User: "+message.Messages.Content)
		}

		if len(dialogx.ChatHistory) > t.chatSaveHistoryLimit {
			dialogx.ChatHistory = dialogx.ChatHistory[len(dialogx.ChatHistory)-t.chatSaveHistoryLimit:]
		}

		// if messageEvent.Intention != "" {
		// 	userInfo.Expert = messageEvent.Intention
		// 	if userInfo.FirstMutil {
		// 		expert.CacheContentIntent(messageEvent.Messages.Content, messageEvent.Intention)
		// 	}
		// }

		if dialogx.Program == "" { // 还没有分配到程序库
			logger.Debug("为其寻找合适的专家")
			var possibleIntentions []PossibleIntentions
			bestProgram, possibleIntentions := t.intentMatch.FindBestIntent(message.Messages.Content, message.Messages.Attachments, !dialogx.Mutil)

			var gotoMutil bool // 是否要走多轮对话
			if dialogx.Mutil { // 如果在多轮中
				if bestProgram != "" && t.commandFirst { // 找到合适的专家并且是命令优先就去走程序库
					gotoMutil = false
					dialogx.Program = bestProgram
				} else { // 否则给多轮对话
					gotoMutil = true
				}
			} else { // 如果不在多轮中
				if bestProgram != "" {
					gotoMutil = false
					dialogx.Program = bestProgram
				} else {
					gotoMutil = true
				}
			}
			if gotoMutil { // 去走多轮对话
				logger.Debug("分配给多轮对话识别")

				// 用户说的第一句话没有匹配到意图，需要多轮识别，仅限一个场景中的第一句话用以存储意图识别识别不到而多轮识别到存入缓存
				dialogx.FirstMutil = !dialogx.Mutil
				dialogx.Mutil = true

				// toChatMessage := ExpertToChatMessage{
				// 	EventType:          1001,
				// 	DialogID:           message.DialogID,
				// 	MessageID:          message.MessageID,
				// 	UserId:             message.UserId,
				// 	PossibleIntentions: possibleIntentions,
				// 	Messages: struct {
				// 		Content     string             `json:"content"`
				// 		Attachments []types.Attachment `json:"attachments"`
				// 		History     []string           `json:"history,omitempty"`
				// 	}{
				// 		Content:     message.Messages.Content,
				// 		Attachments: message.Messages.Attachments,
				// 		History:     dialogx.ChatHistory,
				// 	},
				// }

				toChatMessage := *message
				toChatMessage.PossibleIntentions = possibleIntentions
				toChatMessage.Messages.History = dialogx.ChatHistory

				msg, err := json.Marshal(toChatMessage)
				if err != nil {
					logger.Error("Failed to marshal chat message: %v", err)
				}
				t.chatMessageHandler(toChatMessage, string(msg))
				return
			} else {
				dialogx.FirstMutil = false
				dialogx.Mutil = false

				// toProgramMessage := ExpertToProgramMessage{
				// 	EventType: 1001,
				// 	DialogID:  message.DialogID,
				// 	MessageID: message.MessageID,
				// 	UserId:    message.UserId,
				// 	Intention: dialogx.Program,
				// 	Messages: struct {
				// 		Content     string             `json:"content"`
				// 		Attachments []types.Attachment `json:"attachments"`
				// 	}{
				// 		Content:     message.Messages.Content,
				// 		Attachments: message.Messages.Attachments,
				// 	},
				// }

				toProgramMessage := *message
				toProgramMessage.Intention = dialogx.Program

				msg, err := json.Marshal(toProgramMessage)
				if err != nil {
					logger.Error("Failed to marshal client message: %v", err)
				}
				logger.Debug("分配到专家:", dialogx.Program)
				t.programMessageHandler(toProgramMessage, string(msg))
				return
			}
		} else {
			dialogx.FirstMutil = false
			logger.Debug("继续使用当前程序库:", dialogx.Program)

			// toProgramMessage := ExpertToProgramMessage{
			// 	EventType: 1001,
			// 	DialogID:  message.DialogID,
			// 	MessageID: message.MessageID,
			// 	UserId:    message.UserId,
			// 	Intention: dialogx.Program,
			// 	Messages: struct {
			// 		Content     string             `json:"content"`
			// 		Attachments []types.Attachment `json:"attachments"`
			// 	}{
			// 		Content:     message.Messages.Content,
			// 		Attachments: message.Messages.Attachments,
			// 	},
			// }

			toProgramMessage := *message
			toProgramMessage.Intention = dialogx.Program

			msg, err := json.Marshal(toProgramMessage)
			if err != nil {
				logger.Error("Failed to marshal client message: %v", err)
			}
			logger.Debug("分配到专家:", dialogx.Program)
			t.programMessageHandler(toProgramMessage, string(msg))
			return
		}
	case 1002: // 客户端终止对话
		if dialogx.Program == "" {
			return
		}
		msg, err := json.Marshal(message)
		if err != nil {
			logger.Error("Failed to marshal client message: %v", err)
		}
		toProgramMessage := *message
		t.programMessageHandler(toProgramMessage, string(msg))
		dialogx.Program = ""

	default:
		log.Printf("收到未知事件类型: %d", message.EventType)
	}
}

func (t *Expert) handleFromProgramMessage(message *TotalMessage) {
	logger.Debug("收到程序库消息:", *message)

	dialogx, exists := t.dialogs[message.DialogID]
	if !exists {
		// 用户id 未记录，直接返回
		return
	}
	dialogx.RWMutex.Lock()
	defer dialogx.RWMutex.Unlock()
	switch message.EventType {
	case 2001: // 客户端发送消息

		logger.Infof("【回复用户】:%s", message.Messages.Content)
		dialogx.ChatHistory = append(dialogx.ChatHistory, "Progarm: "+message.Messages.Content)
		// chatHistory 最大保存 t.chatSaveHistoryLimit 条信息
		if len(dialogx.ChatHistory) > t.chatSaveHistoryLimit {
			dialogx.ChatHistory = dialogx.ChatHistory[len(dialogx.ChatHistory)-t.chatSaveHistoryLimit:]
		}

		toUserMessage := *message
		msg, err := json.Marshal(toUserMessage)
		if err != nil {
			logger.Error("Failed to marshal chat message: %v", err)
		}
		t.userMessageHandler(toUserMessage, string(msg))

	case 2002: // 客户端终止对话
		toUserMessage := *message
		msg, err := json.Marshal(toUserMessage)
		if err != nil {
			logger.Error("Failed to marshal chat message: %v", err)
		}
		t.userMessageHandler(toUserMessage, string(msg))
		dialogx.Program = ""

	case 2003: // 专家不支持该能力需要重新分配一个专家
		dialogx.Program = ""
		t.handleFromUserMessage(message)

	default:
		log.Printf("收到未知事件类型: %d", message.EventType)
	}
}

func (t *Expert) handleFromChatMessage(message *TotalMessage) {

	logger.Debug("收到多轮对话:", *message)

	dialogx, exists := t.dialogs[message.DialogID]
	if !exists {
		// 用户id 未注册
		return
	}
	dialogx.RWMutex.Lock()
	defer dialogx.RWMutex.Unlock()
	switch message.EventType {
	case 1001: // 多轮对话总结用户的需求，使用1001 代表用户返回请求专家
		dialogx.Mutil = false
		t.handleFromUserMessage(message)
	case 2001: // 客户端发送消息

		logger.Infof("【回复用户】:%s", message.Messages.Content)
		dialogx.ChatHistory = append(dialogx.ChatHistory, "Chat: "+message.Messages.Content)
		// chatHistory 最大保存20条信息
		if len(dialogx.ChatHistory) > t.chatSaveHistoryLimit {
			dialogx.ChatHistory = dialogx.ChatHistory[len(dialogx.ChatHistory)-t.chatSaveHistoryLimit:]
		}

		toUserMessage := *message
		msg, err := json.Marshal(toUserMessage)
		if err != nil {
			logger.Error("Failed to marshal chat message: %v", err)
		}
		t.userMessageHandler(toUserMessage, string(msg))

	default:
		log.Printf("收到未知事件类型: %d", message.EventType)
	}
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
func (t *Expert) UpdateIntentMatcherFromRNNPath() {
	logger.Info("Updating RNN intent matcher...")
	// 实际逻辑的占位符
	for name := range t.rnnIntent.GetAllRNNIntents() {
		t.UnRegister(name)
	}
	t.rnnIntent = nil
	t.getRNNIntentMangerFromFile()
}
