用 go实现一个程序库聊天系统

1.在 program.go 中补全 case 1001: 时的逻辑，这个地方中会获取不同的用户聊天信息

2.新建一个go代码文件，在该在收到用户的消息后，首先获取用户的dailog_id 和 user_id ,从map 中获取该id的聊天实例，如果没有就去初始化一个,获取用户的意图，实例化相应程序库nodejs实例，和每个nodejs 实例的域套接字均不同，不要混用，这个dailog_id 的用户后续聊天都会直接转给这个实例

3.当一个dialog_id 存在nodejs 运行时，这个dialog_id 的所有对话均直接通过域套接字转给该nodejs 程序

4.nodejs 程序在执行完成后会发送2002 来结束聊天，2003 代表不支持该功能

5.当nodejs 程序运行时如果几个消失都没有新的消息，那么关闭这个nodejs 程序，并发送2002

6.提供http 接口，让nodejs 程序可以调用查询，保存相关数据，并且定时的保存用户的数据，按照每个dialog_id.json 为名存在数据目录中


# nodejs程序库相关定义


## lib 和程序工具通信方式
具体程序 和 lib 库之间通过Unix域套接字来通信，并通过命令行参数将Unix域套接字文件路径传递给js工具。双方通过读写这个Unix域套接字来进行通信。

## 本地程序库具体文件结构
```
├── ProgramPath
│   ├── hello                 // 目录名和意图名
│   |	└── hello.js
```

## 程序工具域套接字文件路径参数
```
--socket=xxx // 输入到具体程序工具的管道
--port=8083
```

## go 程序和 工具管道内消息定义

* 前台对话--前台结束--进程结束
* 前台对话--前台结束--后台运行--进程结束
处于后台时对话的话筒已经还给专家了，此时返回的内容也会返回给专家，专家返回给前置服务，但专家不会对返回的内容进行处理了。

### 消息格式设计
我们采用固定结构的头部 + 可变长度的正文：

```
[消息头部(16字节)] + [消息正文(n字节)] 
```

### 消息头部（共16字节）：
- 魔术标识（4字节）：uint32，魔术标识（大端序）
- 版本号（2字节）：uint16，协议版本号
- 类型标识（2字节）：uint16，标识消息类型
- 正文长度（4字节）：uint32，标识正文的字节数（大端序）
<!-- - 校验码（4字节）：uint32，CRC32校验码（大端序） -->
- 保留字段（4字节）：用于未来扩展

### 消息正文

#### **事件 1001: 收到用户发送正文消息**

**格式:**

```json
{
    "event_type": 1001,
    "dialog_id": "10568594826961410",
    "message_id": "msg-4o49wg0txg",
    "user_id": "user-123456789",
    "messages": {
        "content": "提交代码没有编译出版本，帮我查看一下自动构建的状态",
        "attachments": [
            {
                "type": "file",
                "name": "readme.md",
                "file_id": "file_xxxxxxxxxxxx",
                "option": {}
            }
        ]
    }
}
```

-   `event_type`: `Integer` - 固定为 `1001`。
-   `dialog_id`: `String` - 当前对话的唯一标识符。
-   `message_id`: `String` - 消息的唯一标识符。
-   `messages`: `Object` - 消息内容。
    -   `content`: `String` - 文本消息内容。
    -   `attachments`: `Array` (可选) - 附件列表。
        -   `type`: `String` - 附件类型 (例如: "file")。
        -   `name`: `String` - 文件名。
        -   `file_id`: `String` - 文件的唯一标识符。
        -   `option`: `Object` - 附加选项。

#### **事件 1002: 客户端终止对话**

当客户端（用户）主动关闭对话时，发送此事件。

**格式:**

```json
{
    "event_type": 1002,
    "user_id": "user-123456789",
    "dialog_id": "10568594826961410"
}
```

-   `event_type`: `Integer` - 固定为 `1002`。
-   `dialog_id`: `String` - 需要终止的对话的唯一标识符。

### 2.3. 服务端 -> 客户端 事件 

服务端可以向客户端推送以下类型的事件：

#### **事件 2001: 接收消息**

当程序库回复消息时，服务端会向客户端推送此事件。其结构与客户端发送消息（1001）的结构相同。

**格式:**

```json
{
    "event_type": 2001,
    "dialog_id": "10568594826961410",
    "user_id": "user-123456789",
    "message_id": "msg-4o49wg0txg",
    "messages": {
        "content": "你好",
        "attachments": [
            {
                "type": "file",
                "name": "readme.md",
                "file_id": "file_xxxxxxxxxxxx",
                "option": {}
            }
        ]
    }
}
```

#### **事件 2002: 程序工具，前台结束对话**

当专家主动关闭前台对话时，服务端会推送此事件。话筒会返回给专家

**格式:**

```json
{
    "event_type": 2002,
    "user_id": "user-123456789",
    "dialog_id": "10568594826961410"
}
```

-   `event_type`: `Integer` - 固定为 `2002`。
-   `dialog_id`: `String` - 被终止的对话的唯一标识符。

#### **事件 2003: 工具功能不支持（返回重新选择工具）**

当此工具不支持时，结束当前会话并将控制权交还给主持人（专家）时，工具端会推送此事件。

**格式:**

```json
{
    "event_type": 2003,
    "dialog_id": "10568594826961410",
    "user_id": "user-123456789",
    "message_id": "msg-4o49wg0txg", // 可能需要新加  如果不能处理带上原有的上下文
    "messages": {    //  可能需要新加  如果不能处理带上原有的上下文
        "content": "xxxx",
        "attachments": [
            {
                "type": "file",
                "name": "readme.md",
                "file_id": "file_xxxxxxxxxxxx",
                "option": {}
            }
        ]
    }
}
```

-   `event_type`: `Integer` - 固定为 `2003`。
-   `dialog_id`: `String` - 已结束的对话的唯一标识符。



### 特殊指令

#### 提供指令来查询工具记忆  使用http 来通信来简化使用
```json
{
    "event_type": 3000,  //  固定为 3000 代表特殊指令
    "action": "query_tool_memory",
    "key": "last_release_repo",
    "user_id": "user-123456789",
}
```
对应回复
```json
{
    "event_type": 3000,  //  固定为 3000 代表特殊指令
    "action": "get_tool_memory",
    "key": "last_release_repo",
    "value":"http://xxx.xxx.xx/xxx.release.git",
}
```

#### 提供指令来保存工具记忆  使用http 来通信来简化使用
```json
{
	"event_type": 3000,  //  固定为 3000 代表特殊指令
    "action": "save_tool_memory",
    "key": "last_release_repo",
	"value":"http://xxx.xxx.xx/xxx.release.git",
    "user_id": "user-123456789",
}
```
