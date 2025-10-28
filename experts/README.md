## 专家 接口

```go
NewExpert() *Expert  // 获取专家 实例

(t *Expert) SetDataFilePath(string) // 设置数据卷路径路径  不设置默认用 ~/expert/experts/  支持配置文件设置
(t *Expert) SetRNNIntentPath(string) // 设置本地rnn 意图识别模型路径 不设置默认用 ~/expert/models/ 支持配置文件设置
(t *Expert) SetCommandFirst(bool) // 设置进入多轮对话时命令优先 支持配置文件设置

(t *Expert) HandleUserRequestMessage(jsonx)  // 给专家的消息由此传入  
(t *Expert) HandleUserRequestMessageString(string)  // 给多轮对话的消息由此传入 ,字符串传入，chat 内部会自己解析，和 HandleUserRequestMessage 二选一使用
(t *Expert) SetUserMessageHandler(func(any,string)) // 由此监听专家返回给用户的消息

(t *Expert) HandleToolRequestMessage(jsonx)  // 程序库给专家的消息由此传入
(t *Expert) HandleToolRequestMessageString(string)  // 程序库给专家的消息由此传入,字符串传入，chat 内部会自己解析，和 HandleToolRequestMessage 二选一使用
(t *Expert) SetToToolMessageHandler(func(any,string))  // 回调，当专家返回给程序库消息时，触发此函数  由此监听专家给tool的消息

(t *Expert) HandleChatRequestMessage(jsonx)  // 多轮对话给专家的消息由此传入
(t *Expert) HandleChatRequestMessageString(string)  // 多轮对话给专家的消息由此传入 ,字符串传入，chat 内部会自己解析，和 HandleChatRequestMessage 二选一使用
(t *Expert) SetToChatMessageHandler(func(any,string))  // 回调，当专家返回给多轮对话消息时，触发此函数 由此监听专家给chat的消息

(t *Expert) Run() // 启动程序库实例

(t *Expert) GetAllIntentNames() []string // 获取所有意图名称
(t *Expert) UpdateIntentMatcher()  // 从本地模型路径重新加载意图识别模型

```