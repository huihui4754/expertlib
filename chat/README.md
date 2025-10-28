## chat 多轮对话接口

```go

NewChat() *Chat  // 获取多轮对话 实例

(t *Chat) SetDataFilePath(string) // 设置数据卷路径路径  不设置默认用 ~/expert/chat/ 支持配置文件设置
(t *Chat) SetLLMUrl(string) // 设置大模型链接路径  支持配置文件设置
(t *Chat) SetSystemPrompt(string) // 设置多轮对话个性能力提示词
(t *Chat) SetFunctionCall([]funcall) //

(t *Chat) HandleExpertRequestMessage(jsonx any)  // 给多轮对话的消息由此传入 
(t *Chat) HandleExpertRequestMessageString(string)  // 给多轮对话的消息由此传入 ,字符串传入，chat 内部会自己解析，和 HandleUserRequestMessage 二选一使用
(t *Chat) SetToExpertMessageHandler(func(any,string)) // 由此监听多轮对话返回的消息 ,对象 和 字符串是同样的，

(t *Chat) Run() // 启动多轮对话实例

```