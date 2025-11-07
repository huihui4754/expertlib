package main

import (
	"net/http"

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
	listenAddr   = "0.0.0.0:8084"
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

func main() {
	// 获取程序库实例
	expertx := experts.NewExpert()
	expertx.SetDataFilePath("/home/zhangsh/test/expertdata") // 设置数据卷路径
	expertx.SetRNNIntentPath("/home/zhangsh/test/rnnmodel")  // 设置本地rnn 意图识别模型路径，
	expertx.SetONNXLibPath("/home/zhangsh/test/libonnxruntime.so.1.22.0")

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
	chatx.SetToExpertMessageHandler(func(_ types.TotalMessage, message string) {
		expertx.HandleChatRequestMessage(message)
	})

	expertx.SetToChatMessageHandler(func(_ types.TotalMessage, message string) {
		logger.Debugf("转发给多轮对话： %v", message)
		chatx.HandleExpertRequestMessage(message)
	})

	go funclibs.Run() // 启动程序库实例
	go chatx.Run()    // 启动多轮对话实例
	go expertx.Run()  // 启动专家实例

	http.HandleFunc(serverPrefix+"/opendialog", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(expertx, w, r)
	})

	// listenAddr := fmt.Sprintf(":%d", config.Port) // 自由设置端口
	logger.Debug("start websocket")
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}
