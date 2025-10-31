## 专家 接口

```go
SetLogger(int) // 设置日志级别

NewExpert() *Expert  // 获取专家 实例

(t *Expert) SetChatSaveHistoryLimit(int) // 设置多轮对话保存的历史消息条数限制
(t *Expert) SetMessageFormatFunc(func(string) string) // 设置消息处理后再送入意图识别器的格式化函数
(t *Expert) Register(func() IntentMatchInter, string) // 注册意图匹配器
(t *Expert) UnRegister(string) // 注销某个意图匹配器

(t *Expert) SetDataFilePath(string) // 设置数据卷路径路径  不设置默认用 ~/expert/experts/  支持配置文件设置
(t *Expert) SetRNNIntentPath(string) // 设置本地rnn 意图识别模型路径 不设置默认用 ~/expert/models/ 支持配置文件设置
(t *Expert) SetCommandFirst(bool) // 设置进入多轮对话时命令优先 支持配置文件设置
(t *Expert) SetONNXLibPath(string) // 设置ONNX动态库文件路径,onnxruntime 动态库需要下载并指明路径
(t *Expert) SetSaveIntervalTime(time.Duration) // 设置保存dialog信息和意图保存的时间间隔

(t *Expert) SetSaveDialogInfoHandler(func(map[string]*DialogInfo)) // 设置保存dialog信息的处理函数
(t *Expert) SetLoadDialogInfoHandler(func() map[string]*DialogInfo) // 设置加载dialog信息的处理函数

(t *Expert) HandleUserRequestMessage(any)  // 给专家的消息由此传入，支持 TotalMessage ， string ,[]byte 等多种类型
(t *Expert) SetToUserMessageHandler(func(TotalMessage, string)) // 由此监听专家返回给用户的消息

(t *Expert) HandleProgramRequestMessage(any)  // 程序库给专家的消息由此传入
(t *Expert) SetToProgramMessageHandler(func(TotalMessage, string))  // 回调，当专家返回给程序库消息时，触发此函数

(t *Expert) HandleChatRequestMessage(any)  // 多轮对话给专家的消息由此传入
(t *Expert) SetToChatMessageHandler(func(TotalMessage, string))  // 回调，当专家返回给多轮对话消息时，触发此函数

(t *Expert) Run() // 启动程序库实例

(t *Expert) GetAllIntentNames() []string // 获取所有意图名称
(t *Expert) UpdateIntentMatcherFromRNNPath()  // 从本地rnn 路径重新加载所有rnn 模型，用于增加或删除意图识别后更新使用
```