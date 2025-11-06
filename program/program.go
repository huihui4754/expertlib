package programs

import (
	"encoding/json"
	"fmt"
	"log"
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
	expertMessageHandler   func(any, string)
	expertMessageInChan    chan *TotalMessage
	toExpertMessageOutChan chan *TotalMessage
}

func NewTool() *program {
	defalutProgramPath := ""
	defalutDataPath := ""
	currentUser, err := user.Current() // todo 后续支持从配置文件读取配置
	if err != nil {
		fmt.Printf("获取用户信息失败：%v\n", err)
	} else {
		defalutProgramPath = filepath.Join(currentUser.HomeDir, "expert", "program")
		defalutDataPath = filepath.Join(currentUser.HomeDir, "expert", "js")
	}
	return &program{
		dataFilePath:           defalutDataPath,
		programPath:            defalutProgramPath,
		expertMessageInChan:    make(chan *TotalMessage),
		toExpertMessageOutChan: make(chan *TotalMessage),
	}
}

func (p *program) SetDataFilePath(path string) {
	p.dataFilePath = path
	logger.Info("Data file path set to:", path)
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
	if p.expertMessageInChan != nil && messagePointer != nil && err == nil {
		p.expertMessageInChan <- messagePointer
	}
}

func (p *program) SetToExpertMessageHandler(handler func(any, string)) {
	p.expertMessageHandler = handler
	logger.Info("ExpertMessageHandler set")
}

func (p *program) handleFromExpertMessage(message *TotalMessage) {

	switch message.EventType {
	case 1001:
		//  todo 这里需要去启动相应的程序库来对话

	case 1002:
		logger.Debug("专家终止对话")

	default:
		log.Printf("收到未知事件类型: %d", message.EventType)
	}

}

func (p *program) Run() {

	logger.Info("Program instance running")
	for {
		select {
		case expertMsg := <-p.expertMessageInChan:
			go p.handleFromExpertMessage(expertMsg)
		case toExpertMsg := <-p.toExpertMessageOutChan:
			go func() {
				toExpertMessage := *toExpertMsg
				msg, err := json.Marshal(toExpertMessage)
				if err != nil {
					logger.Error("Failed to marshal chat message: %v", err)
				}
				p.expertMessageHandler(toExpertMessage, string(msg))
			}()
		}
	}

}

func (p *program) GetProgramNames() []string {
	logger.Debug("Getting all program names")
	// Placeholder for actual logic
	return []string{"program1", "program2"}
}

func (p *program) UpdatePrograms() {
	logger.Info("Updating program")
	// Placeholder for actual logic
}
