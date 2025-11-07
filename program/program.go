package programs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/user"
	"path/filepath"

	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
)

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

type TotalMessage = types.TotalMessage

func SetLogger(level loglevel.Level) {
	logger.SetLevel(level)
}

type program struct {
	dataFilePath           string
	programPath            string
	expertMessageHandler   func(TotalMessage, string)
	expertMessageInChan    chan *TotalMessage
	toExpertMessageOutChan chan *TotalMessage
	sessionManager         *SessionManager
	dataStorage            *StorageManager
	port                   string
}

func NewTool() *program {
	defalutProgramPath := ""
	defalutDataPath := ""
	defalutPort := "8765"
	currentUser, err := user.Current() // todo 后续支持从配置文件读取配置
	if err != nil {
		fmt.Printf("获取用户信息失败：%v\n", err)
	} else {
		defalutProgramPath = filepath.Join(currentUser.HomeDir, "expert", "js")
		defalutDataPath = filepath.Join(currentUser.HomeDir, "expert", "program")
	}

	// Initialize storage with the data file path
	dataStorage := NewStorage(defalutDataPath, defalutPort)

	toExpertChan := make(chan *TotalMessage)

	return &program{
		dataFilePath:           defalutDataPath,
		programPath:            defalutProgramPath,
		expertMessageInChan:    make(chan *TotalMessage),
		toExpertMessageOutChan: toExpertChan,
		sessionManager:         NewSessionManager(toExpertChan),
		dataStorage:            dataStorage,
		port:                   defalutPort,
	}
}

func (p *program) SetDataFilePath(path string) {
	p.dataFilePath = path
	p.dataStorage.DataDirPath = path
	logger.Info("Data file path set to:", path)
	// Update storage manager with new path if it's already initialized
}

func (p *program) SetProgramPath(path string) {
	p.programPath = path
	logger.Info("Program path set to:", path)
}

func (p *program) HandleExpertRequestMessage(message any) {
	logger.Debugf("Handling Expert request message: %v", message)
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
			return
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
	if p.expertMessageInChan != nil && messagePointer != nil && err == nil {
		p.expertMessageInChan <- messagePointer
	}
}

func (p *program) SetToExpertMessageHandler(handler func(TotalMessage, string)) {
	p.expertMessageHandler = handler
	logger.Info("ExpertMessageHandler set")
}

func (p *program) handleFromExpertMessage(message *TotalMessage) {

	switch message.EventType {
	case types.EventUserMessage: // 1001
		logger.Debugf("Received user message for dialog: %s", message.DialogID)
		if message.Intention == "" {
			return
		}
		session, err := p.sessionManager.GetOrCreateSession(message.DialogID, message.UserId, message.Intention, p.port)
		if err != nil {
			logger.Errorf("Failed to get or create session for dialog %s: %v", message.DialogID, err)
			// 向专家发回ToolNotSupport消息
			p.sendToolNotSupported(message)
			return
		}
		if err := session.Send(message); err != nil {
			logger.Errorf("Failed to send message to nodejs process for dialog %s: %v", message.DialogID, err)
			// 处理通信错误，可能关闭会话并通知专家
			p.sessionManager.CloseSession(message.DialogID, types.EventToolFinish)
			p.sendProgramEnd(message)
		}

	case types.EventClientTerminate: // 1002
		logger.Debugf("Received client terminate for dialog: %s", message.DialogID)
		p.sessionManager.CloseSession(message.DialogID, types.EventClientTerminate)

	default:
		logger.Warnf("收到未知事件类型: %d", message.EventType)
	}

}

func (p *program) sendToolNotSupported(originalMsg *TotalMessage) {
	// Create a ToolNotSupport message and send it back
	notSupportMsg := &TotalMessage{
		EventType: types.EventToolNotSupport,
		DialogID:  originalMsg.DialogID,
		UserId:    originalMsg.UserId,
	}
	p.toExpertMessageOutChan <- notSupportMsg
}

func (p *program) sendProgramEnd(originalMsg *TotalMessage) {
	// Create a ToolNotSupport message and send it back
	endMsg := &TotalMessage{
		EventType: types.EventToolFinish,
		DialogID:  originalMsg.DialogID,
		UserId:    originalMsg.UserId,
	}
	p.toExpertMessageOutChan <- endMsg
}

func (p *program) Run() {

	logger.Info("Program instance running")

	for {
		select {
		case expertMsg := <-p.expertMessageInChan:
			go p.handleFromExpertMessage(expertMsg)
		case toExpertMsg := <-p.toExpertMessageOutChan:
			go func() {
				// The message from session manager is already a complete TotalMessage
				// We just need to marshal it for the handler
				msgBytes, err := json.Marshal(toExpertMsg)
				if err != nil {
					logger.Errorf("Failed to marshal outgoing message: %v", err)
					return
				}
				if p.expertMessageHandler != nil {
					p.expertMessageHandler(*toExpertMsg, string(msgBytes))
				}
			}()
		}
	}

}

func (p *program) GetProgramNames() []string {
	logger.Debug("Getting all program names")
	// Placeholder for actual logic
	return p.sessionManager.GetAllProgramName()
}

func (p *program) GetStroageHandler() func(w http.ResponseWriter, r *http.Request) {
	return p.dataStorage.GetStroageHandler()
}

func (p *program) RunStroageUserData() {
	go p.dataStorage.RunHTTPServer() // Start the HTTP server for storage
}
