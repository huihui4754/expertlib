
# ExpertLib 文档

## 1. 项目概述

ExpertLib 是一个 Go 语言库，用于构建模块化和可扩展的对话式 AI 应用程序。它旨在理解用户意图并将任务分派给由专业“专家”和“程序”组成的网络。该库利用大型语言模型（LLM）进行自然语言理解，并且可以通过用 Node.js 编写的自定义逻辑进行扩展。

ExpertLib 的架构基于以下关键原则：

*   **模块化**：系统分为具有明确职责的不同组件（`chat`、`experts`、`program`）。
*   **可扩展性**：通过创建新的“专家”（用于意图匹配）和“程序”（用于任务执行），可以轻松添加新功能。
*   **意图驱动**：系统的核心是意图匹配引擎，它将用户请求路由到适当的处理程序。
*   **LLM 驱动**：该库使用 LLM 进行高级自然语言理解，包括意图识别和函数调用。

## 2. 核心组件

### 2.1. `chat`

`chat` 包负责管理与大型语言模型（LLM）的交互。

*   **`Chat`**：管理聊天会话的主要结构体。它处理传入和传出消息，并使用 `LLMChatWithFunCallManager` 与 LLM 交互。
*   **`LLMChatWithFunCallManager`**：管理与 LLM 的多个聊天会话。它存储聊天历史记录并处理 `ChatLLM` 方法，该方法使用可能的意图和聊天历史记录构建提示并调用 LLM。
*   **`OpenaiChatLLM`**：实现与 OpenAI 兼容的 LLM 的交互。它向 LLM 发送请求，包括对话历史记录和可用工具，并处理来自 LLM 的函数调用。

### 2.2. `experts`

`experts` 包实现了系统的核心业务逻辑。它接收用户输入，识别用户意图，并将任务分派给适当的“程序”或“聊天”模块。

*   **`Expert`**：管理对话并处理来自用户、程序和聊天模块消息的中心结构体。它使用 `IntentMatchManager` 来确定用户意图。
*   **`IntentMatchManager`**：管理和匹配用户意图。它维护一个意图匹配器注册表，并使用缓存来加速意图识别。
*   **`RNNIntent`**：使用循环神经网络（RNN）模型实现意图识别。它使用 `onnxruntime_go` 库运行 ONNX 模型进行推理。

### 2.3. `program`

`program` 包实现了一个插件系统，其中每个“程序”都是一个独立的 Node.js 应用程序。

*   **`program`**：“程序”模块的入口点。它接收来自 `expert` 模块的消息，并根据 `Intention`（意图）将任务分派给适当的会话。
*   **`SessionManager` 和 `Session`**：`SessionManager` 创建、管理和关闭会话。每个 `Session` 代表与特定意图（即 Node.js 程序）的交互。当创建新会话时，它会启动相应的 Node.js 子进程并通过 Unix 套接字与其通信。
*   **`StorageManager`**：为 Node.js 程序提供简单的键值存储服务。它通过 HTTP 服务器公开 `/memory` 端点，允许程序通过 POST 请求保存数据，通过 GET 请求查询数据。

### 2.4. `types`

`types` 包定义了整个系统使用的核心数据结构和消息格式。

*   **`DialogInfo`**：存储每个对话会话的状态。
*   **`FunctionCall`**：定义与 LLM 进行函数调用的结构体。
*   **`TotalMessage`**：一个通用消息结构体，可以根据 `EventType` 字段表示不同类型的消息。
*   **`MessageHeader`**：定义 `program` 模块和 Node.js 子进程之间交换消息的头部。

### 2.5. `ssemcpclient`

`ssemcpclient` 包实现了使用服务器发送事件（SSE）进行通信的客户端。

*   **`MCPSSEClient`**：管理与 SSE 服务器的连接，处理事件，并提供在服务器上调用函数的方法。
*   **`SSEFuncCall`**：一个适配器，用于将 SSE 客户端用作 LLM 的函数调用机制。它从 SSE 服务器获取可用工具，将其转换为 LLM 可以理解的格式，并提供 `CallTool` 方法来执行函数调用。

## 3. 工作流程

1.  用户向系统发送消息。
2.  `Expert.HandleUserRequestMessage` 方法接收消息。
3.  `Expert` 使用 `IntentMatchManager` 为用户消息找到最佳意图。
4.  如果找到合适的意图，`Expert` 将消息连同识别出的意图转发给 `program` 模块。
5.  如果没有找到合适的意图，`Expert` 将消息转发给 `chat` 模块，进行与 LLM 的多轮对话。
6.  `program` 模块接收消息，并根据意图启动一个新的 Node.js 程序（如果该意图的会话尚不存在）。
7.  Node.js 程序执行所需的任务，可能会使用 `StorageManager` 存储或检索数据。
8.  Node.js 程序将结果发送回 `program` 模块，然后 `program` 模块将其转发给 `Expert`。
9.  `Expert` 将最终响应发送给用户。

## 4. 如何使用

expert ，chat ,program 三个模块可以程序中单独使用，也可以写在一个程序中绑定在一起：

1.  **创建一个 `Expert` 实例，并设置相关参数**：
    ```go
    expert := experts.NewExpert()
    expert.SetDataFilePath("xxx/your_data_dir") // 设置保存数据的本地目录，不设置默认为用户目录下的 expert/dialog/ 目录
    expert.SetRNNIntentPath("xxx/your_rnnmodel_dir") // 设置RNN Intent模型路径。不设置默认使用 目录下的 expert/rnnmodel/ 目录
    expert.SetONNXLibPath("xxx/your_onnxruntime_lib") // 设置onnx runtime 动态库路径。如果要使用本地rnn 模型意图识别，必须下载相关库并调用该函数设置
    ```
2.  **创建一个 `Chat` 实例**：
    ```go
    chatx := chat.NewChat()
    chatx.SetDataPath("xxx/your_chat_data_dir") // 设置数据卷路径
    chatx.SetLLMUrl("http://xxx.xxx.xx/xxxx/v1") // 设置大模型链接路径
    chatx.SetSystemPrompt("你是某方面的专家能处理 xxx 的问题") // 设置多轮对话个性能力提示词
    ```
3.  **创建一个 `program` 实例**：
    ```go
    program := programs.NewTool()
    program.SetDataPath("xxx/your_program_data_dir") // 设置数据卷路径
    program.SetProgramPath("xxx/your_program_js_dir") // 设置本地js 程序库路径
    ```

4.  **注册意图匹配器**：
    ```go
    expert.Register(myIntentMatcher, "controlAutoBuild")
    ```
5.  **设置消息处理程序**：
    ```go
    expert.SetToUserMessageHandler(func(msg types.TotalMessage, s string) {
        // 将消息发送给用户
    })
    expert.SetToProgramMessageHandler(program.HandleExpertRequestMessage)
    expert.SetToChatMessageHandler(chat.HandleExpertRequestMessage)

    chat.SetToExpertMessageHandler(expert.HandleChatRequestMessage)
    program.SetToExpertMessageHandler(expert.HandleProgramRequestMessage)
    ```
6.  **运行实例**：
    ```go
    go expert.Run()
    go chat.Run()
    go program.Run()
    ```
7.  **处理用户请求**：
    ```go
    expert.HandleUserRequestMessage(userMessage)
    ```

## 5. 如何扩展

### 5.1 添加新的rnn神经网络意图识别 （）
要添加新的rnn 神经网络意图识别参照 https://git.ipanel.cn/faas/zhangshy/neural-network-skill 仓库说明

### 5.2. 添加新的意图识别（代码中添加）

要添加新的意图，您需要创建一个实现 `IntentMatchInter` 接口的结构体：

```go
type MyIntentMatcher struct{}

func (m *MyIntentMatcher) GetIntentName() string {
    return "myIntent"
}

func (m *MyIntentMatcher) GetIntentDesc() string {
    return "这是我的自定义意图。"
}

func (m *MyIntentMatcher) Matching(content string, attachments []types.Attachment) float64 {
    // 您的意图匹配逻辑在此处
    // 返回 0.0 到 1.0 之间的分数
}
```

然后，将其注册到 `Expert` 实例：

```go
expert.Register(func() experts.IntentMatchInter {
    return &MyIntentMatcher{}
}, "myIntent")
```

### 5.3. 添加新的程序

要添加新的程序，您需要创建一个 Node.js 脚本，该脚本通过 Unix 套接字与 `program` 模块通信。该脚本将接收来自 `program` 模块的消息，并可以发送消息回去。

Node.js 脚本应放置在为 `program` 实例配置的 `programPath` 下的目录中。目录名称和脚本名称应与意图名称匹配。例如，对于名为 `myIntent` 的意图，脚本应位于 `<programPath>/myIntent/myIntent.js`。

Node.js 脚本将接收套接字路径和数据端口作为命令行参数。然后，它可以使用 `net` 等库连接到套接字，并使用 `axios` 与存储服务器通信。
