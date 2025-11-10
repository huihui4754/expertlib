package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openai/openai-go/v3"

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
	urlRegex     = regexp.MustCompile(`(https?://[^\s]+\.release.git)`)
	tagRegex     = regexp.MustCompile(`([a-zA-Z0-9]+-v\d+\.\d+|v\d+\.\d+)`)
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

type CheckAutoStatus struct {
}

func (w *CheckAutoStatus) GetIntentName() string {
	return "checkAutoStatus" // 注册的意图名称，和 对应的 nodejs 脚本名称要保持一致
}

func (w *CheckAutoStatus) GetIntentDesc() string {
	return "查看自动构建的状态"
}

// 定义匹配规则、置信度及澄清问题
// 规则按优先级（置信度）从高到低排序
var workDocIntentRules = []struct {
	Rule      string
	Certainty float64
	Question  string // 用于低置信度匹配时的澄清问题
}{
	// --- 排除规则 (P = 0): 明确排除 ---
	{Rule: `.*(不|别|无须|不用).*(查看|检查|看).*(自动构建|构建).*(状态).*`, Certainty: 0},

	// --- 高置信度 (P > 0.8): 明确指令 ---
	{Rule: `.*(查看|检查|看).*(自动构建|构建).*(状态).*`, Certainty: 0.95},
}

func (w *CheckAutoStatus) Matching(content string, attachments []Attachment) float64 {
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

func NewCheckAutoStatus() experts.IntentMatchInter {
	return &CheckAutoStatus{}
}

func getSumTool(a, b int) int {
	return a + b
}

func main() {
	// 获取程序库实例
	expertx := experts.NewExpert()
	expertx.SetDataFilePath("/home/zhangsh/test/expertdata") // 设置数据卷路径
	expertx.SetRNNIntentPath("/home/zhangsh/test/rnnmodel")  // 设置本地rnn 意图识别模型路径，
	expertx.SetONNXLibPath("/home/zhangsh/test/libonnxruntime.so.1.22.0")

	expertx.Register(NewCheckAutoStatus, "checkAutoStatus")
	expertx.SetCommandFirst(true)

	expertx.SetMessageFormatFunc(func(s string) string {

		content := urlRegex.ReplaceAllString(s, "")
		content = strings.TrimSpace(tagRegex.ReplaceAllString(content, ""))
		return content
	})

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
	funclibs.SetSaveIntervalTime(1 * time.Minute)
	funclibs.SetToExpertMessageHandler(func(_ types.TotalMessage, message string) {
		expertx.HandleProgramRequestMessage(message)
	})

	expertx.SetToProgramMessageHandler(func(_ types.TotalMessage, message string) {
		funclibs.HandleExpertRequestMessage(message)
	})

	chatx := chat.NewChat()
	chatx.SetDataFilePath("/home/zhangsh/test/chatdata") // 设置数据卷路径
	chatx.SetLLMUrl("http://192.168.101.130:8010/v1")    // 设置大模型链接路径
	chatx.SetModelName("Qwen3-32B-AWQ")                  // 设置大模型链接路径
	chatx.SetSystemPrompt("你是一个有用的ai 助手")                // 设置多轮对话个性能力提示词
	chatx.SetSaveIntervalTime(1 * time.Minute)

	chatx.SetOpenaiChatCompletionToolUnionParam([]openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "add_sum",
			Description: openai.String("计算两个数的相加 "),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"a": map[string]string{
						"type": "number",
					},
					"b": map[string]string{
						"type": "number",
					},
				},
				"required": []string{"a", "b"},
			},
		}),
	})

	chatx.SetCallFunctionHandler(func(call *chat.FunctionCall) (string, error) {
		var err error
		var result = ""
		switch call.Name {
		case "get_weather":
			a := call.Arguments["a"].(int)
			b := call.Arguments["b"].(int)
			sum := getSumTool(a, b)
			result = fmt.Sprintf("两数相加的结果为 %v", sum)
		default:
			err = fmt.Errorf("no support function  tool : %s", call.Name)
		}
		return result, err
	})

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
