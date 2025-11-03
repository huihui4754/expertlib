## chat 多轮对话接口

```go

NewChat() *Chat  // 获取多轮对话 实例

(t *Chat) SetDataFilePath(string) // 设置数据卷路径路径  不设置默认用 ~/expert/chat/ 支持配置文件设置
(t *Chat) SetLLMUrl(string) // 设置大模型链接路径  支持配置文件设置
(t *Chat) SetLLMModelName(string) // 设置使用的大模型名称  支持配置文件设置
(t *Chat) SetRequestLLMHeaders(string)  
(t *Chat) SetSystemPrompt(string) // 设置多轮对话个性能力提示词
(t *Chat) SetFunctionCall([]funcall) // 设置大模型可以使用的 function call
(t *Chat) SetCallFunctionHandler([]funcall)


(t *Tool) HandleExpertRequestMessage(any)  // 给多轮对话的消息由此传入，支持 TotalMessage ， string ,[]byte 等多种类型
(t *Tool) SetToExpertMessageHandler(func(TotalMessage,string))  // 由此监听多轮对话返回的消息

(t *Chat) Run() // 启动多轮对话实例

```