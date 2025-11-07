package main

import (
	"net/http"
	"regexp"
	"time"

	"github.com/gorilla/websocket"

	"github.com/huihui4754/expertlib/chat"
	"github.com/huihui4754/expertlib/experts"
	tools "github.com/huihui4754/expertlib/program"
	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
)

var (
	UserConn *websocket.Conn
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	logger       = loglevel.NewLog(loglevel.Debug)
	serverPrefix = "/api"
	listenAddr   = "0.0.0.0:8085"
)

func handleWebSocket(expertx *experts.Expert, w http.ResponseWriter, r *http.Request) {
	var err error
	UserConn, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("升级连接失败:", err)
		return
	}

	go func() {
		defer UserConn.Close()
		for {
			messageType, message, err := UserConn.ReadMessage()
			if err != nil {
				logger.Error("读取消息失败:", err)
				break
			}
			if messageType == websocket.TextMessage {
				logger.Debug("接收到文本消息:", string(message))
				//  收到字符串消息  调用程序库接口 处理
				expertx.HandleUserRequestMessage(string(message))
			}
		}
	}()
}

type Attachment = types.Attachment

type WorkDocExpertMatch struct {
}

func (w *WorkDocExpertMatch) GetIntentName() string {
	return "checkAutoStatus" // 注册的意图名称，和 对应的 nodejs 脚本名称要保持一致
}

func (w *WorkDocExpertMatch) GetIntentDesc() string {
	return "能够收录一篇公开网文到个人文档"
}

// 定义匹配规则、置信度及澄清问题
// 规则按优先级（置信度）从高到低排序
var workDocIntentRules = []struct {
	Rule      string
	Certainty float64
	Question  string // 用于低置信度匹配时的澄清问题
}{
	// --- 排除规则 (P = 0): 明确排除 ---
	{Rule: `.*(不|别|无须|不用).*(收录|保存|添加|放入|收藏).*`, Certainty: 0},

	// --- 高置信度 (P > 0.8): 明确指令 ---
	{Rule: `.*收录.*`, Certainty: 0.95},
	{Rule: `.*(保存|添加|放入|收藏)到.*(文档|个人文档).*`, Certainty: 0.85},
}

func (w *WorkDocExpertMatch) Matching(content string, attachments []Attachment) float64 {
	for _, rule := range workDocIntentRules {
		// 特殊规则：处理需要附件的场景
		if rule.Certainty == 80 {
			if len(attachments) == 0 {
				continue // 如果没有附件，则跳过此规则
			}
		}

		matched, _ := regexp.MatchString(rule.Rule, content)
		if matched {
			// 如果匹配到排除规则，立即返回0
			if rule.Certainty == 0 {
				return 0
			}
			// 如果匹配到低置信度规则，返回置信度和用于澄清的默认问题
			if rule.Question != "" {
				return rule.Certainty
			}
			// 否则，返回高或中置信度，以及一个空字符串
			return rule.Certainty
		}
	}

	return 0
}

func NewWorkDocExpertMatch() experts.IntentMatchInter {
	return &WorkDocExpertMatch{}
}

func main() {
	// 获取程序库实例
	expertx := experts.NewExpert()
	expertx.SetDataFilePath("/home/zhangsh/test/expertdata") // 设置数据卷路径
	expertx.SetRNNIntentPath("/home/zhangsh/test/rnnmodel")  // 设置本地rnn 意图识别模型路径，
	expertx.SetONNXLibPath("/home/zhangsh/test/libonnxruntime.so.1.22.0")

	expertx.Register(NewWorkDocExpertMatch, "embody_articles")

	expertx.SetToUserMessageHandler(func(_ types.TotalMessage, message string) {
		// 处理程序库返回的消息
		logger.Debug("接收到返回给用户的消息:", message)
		if UserConn != nil {
			err := UserConn.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				logger.Error("发送消息失败:", err)
			}
		}
	})

	funclibs := tools.NewTool()
	funclibs.SetDataFilePath("/home/zhangsh/test/programdata") // 设置数据卷路径
	funclibs.SetProgramPath("/home/zhangsh/test/programjs")    // 设置本地js 程序库路径

	funclibs.SetToExpertMessageHandler(func(_ types.TotalMessage, message string) {
		expertx.HandleProgramRequestMessage(message)
	})

	expertx.SetToProgramMessageHandler(func(_ types.TotalMessage, message string) {
		funclibs.HandleExpertRequestMessage(message)
	})

	chatx := chat.NewChat()
	chatx.SetDataFilePath("/home/zhangsh/test/chatdata") // 设置数据卷路径
	chatx.SetLLMUrl("http://192.168.101.130:8011/v1")    // 设置大模型链接路径
	chatx.SetModelName("Qwen3-8B-AWQ")                   // 设置大模型链接路径
	chatx.SetSystemPrompt("你能够处理自动构建相关的问题")              // 设置多轮对话个性能力提示词
	chatx.SetSaveIntervalTime(time.Minute)
	chatx.SetToExpertMessageHandler(func(_ types.TotalMessage, message string) {
		expertx.HandleChatRequestMessage(message)
	})

	expertx.SetToChatMessageHandler(func(_ types.TotalMessage, message string) {
		logger.Debugf("转发给多轮对话： %v", message)
		chatx.HandleExpertRequestMessage(message)
	})

	go funclibs.Run() // 启动程序库实例
	go funclibs.RunStroageUserData()
	go chatx.Run()   // 启动多轮对话实例
	go expertx.Run() // 启动专家实例

	http.HandleFunc(serverPrefix+"/opendialog", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(expertx, w, r)
	})

	// listenAddr := fmt.Sprintf(":%d", config.Port) // 自由设置端口
	logger.Debug("start websocket")
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}
