
# ExpertLib Documentation

## 1. Project Overview

ExpertLib is a Go library for building modular and extensible conversational AI applications. It is designed to understand user intent and dispatch tasks to a network of specialized "experts" and "programs". The library leverages Large Language Models (LLMs) for natural language understanding and can be extended with custom logic written in Node.js.

The architecture of ExpertLib is based on the following key principles:

*   **Modularity**: The system is divided into distinct components (`chat`, `experts`, `program`) with clear responsibilities.
*   **Extensibility**: New functionalities can be easily added by creating new "experts" (for intent matching) and "programs" (for task execution).
*   **Intent-driven**: The core of the system is an intent matching engine that routes user requests to the appropriate handler.
*   **LLM-powered**: The library uses LLMs for advanced natural language understanding, including intent recognition and function calling.

## 2. Core Components

### 2.1. `chat`

The `chat` package is responsible for managing the interaction with the Large Language Model (LLM).

*   **`Chat`**: The main struct that manages a chat session. It handles incoming and outgoing messages and uses the `LLMChatWithFunCallManager` to interact with the LLM.
*   **`LLMChatWithFunCallManager`**: Manages multiple chat sessions with the LLM. It stores chat history and handles the `ChatLLM` method, which builds a prompt with possible intents and chat history and calls the LLM.
*   **`OpenaiChatLLM`**: Implements the interaction with an OpenAI-compatible LLM. It sends requests to the LLM, including the conversation history and available tools, and handles function calls from the LLM.

### 2.2. `experts`

The `experts` package implements the core business logic of the system. It receives user input, identifies the user's intent, and dispatches the task to the appropriate "program" or "chat" module.

*   **`Expert`**: The central struct that manages dialogs and handles messages from users, programs, and the chat module. It uses an `IntentMatchManager` to determine the user's intent.
*   **`IntentMatchManager`**: Manages and matches user intents. It maintains a registry of intent matchers and uses a cache to speed up intent recognition.
*   **`RNNIntent`**: An implementation of an intent recognizer using a Recurrent Neural Network (RNN) model. It uses the `onnxruntime_go` library to run ONNX models for inference.

### 2.3. `program`

The `program` package implements a plugin system where each "program" is an independent Node.js application.

*   **`program`**: The entry point for the "program" module. It receives messages from the `expert` module and dispatches tasks to the appropriate session based on the `Intention` (intent).
*   **`SessionManager` and `Session`**: The `SessionManager` creates, manages, and closes sessions. Each `Session` represents an interaction with a specific intent (i.e., a Node.js program). When a new session is created, it starts a corresponding Node.js child process and communicates with it via a Unix socket.
*   **`StorageManager`**: Provides a simple key-value storage service for the Node.js programs. It exposes a `/memory` endpoint via an HTTP server, allowing programs to save and query data.

### 2.4. `types`

The `types` package defines the core data structures and message formats used throughout the system.

*   **`DialogInfo`**: Stores the state of each dialog session.
*   **`FunctionCall`**: Defines the structures for function calling with the LLM.
*   **`TotalMessage`**: A generic message structure that can represent different types of messages based on the `EventType` field.
*   **`MessageHeader`**: Defines the header for messages exchanged between the `program` module and the Node.js child processes.

### 2.5. `ssemcpclient`

The `ssemcpclient` package implements a client for a server that uses Server-Sent Events (SSE) for communication.

*   **`MCPSSEClient`**: Manages the connection to the SSE server, handles events, and provides a way to call functions on the server.
*   **`SSEFuncCall`**: An adapter to use the SSE client as a function-calling mechanism for the LLM. It fetches the available tools from the SSE server, converts them into a format that the LLM can understand, and provides a `CallTool` method to execute the function calls.

## 3. Workflow

1.  A user sends a message to the system.
2.  The `Expert.HandleUserRequestMessage` method receives the message.
3.  The `Expert` uses the `IntentMatchManager` to find the best intent for the user's message.
4.  If a suitable intent is found, the `Expert` forwards the message to the `program` module with the identified intent.
5.  If no suitable intent is found, the `Expert` forwards the message to the `chat` module for a multi-turn conversation with the LLM.
6.  The `program` module receives the message and, based on the intent, starts a new Node.js program (if a session for that intent doesn't already exist).
7.  The Node.js program executes the required task, potentially using the `StorageManager` to store or retrieve data.
8.  The Node.js program sends the result back to the `program` module, which then forwards it to the `Expert`.
9.  The `Expert` sends the final response to the user.

## 4. How to Use

To use ExpertLib, you need to:

1.  **Create an `Expert` instance**:
    ```go
    expert := experts.NewExpert()
    ```
2.  **Create a `Chat` instance**:
    ```go
    chat := chat.NewChat()
    ```
3.  **Create a `program` instance**:
    ```go
    program := programs.NewTool()
    ```
4.  **Configure the instances**: Set the data file paths, LLM URL, model name, etc.
5.  **Register intent matchers**:
    ```go
    expert.Register(myIntentMatcher, "myIntent")
    ```
6.  **Set message handlers**:
    ```go
    expert.SetToUserMessageHandler(func(msg types.TotalMessage, s string) {
        // Send the message to the user
    })
    expert.SetToProgramMessageHandler(program.HandleExpertRequestMessage)
    expert.SetToChatMessageHandler(chat.HandleExpertRequestMessage)

    chat.SetToExpertMessageHandler(expert.HandleChatRequestMessage)
    program.SetToExpertMessageHandler(expert.HandleProgramRequestMessage)
    ```
7.  **Run the instances**:
    ```go
    go expert.Run()
    go chat.Run()
    go program.Run()
    ```
8.  **Handle user requests**:
    ```go
    expert.HandleUserRequestMessage(userMessage)
    ```

## 5. How to Extend

### 5.1. Adding a New Expert (Intent)

To add a new intent, you need to create a struct that implements the `IntentMatchInter` interface:

```go
type MyIntentMatcher struct{}

func (m *MyIntentMatcher) GetIntentName() string {
    return "myIntent"
}

func (m *MyIntentMatcher) GetIntentDesc() string {
    return "This is my custom intent."
}

func (m *MyIntentMatcher) Matching(content string, attachments []types.Attachment) float64 {
    // Your intent matching logic here
    // Return a score between 0.0 and 1.0
}
```

Then, register it with the `Expert` instance:

```go
expert.Register(func() experts.IntentMatchInter {
    return &MyIntentMatcher{}
}, "myIntent")
```

### 5.2. Adding a New Program

To add a new program, you need to create a Node.js script that communicates with the `program` module via a Unix socket. The script will receive messages from the `program` module and can send messages back.

The Node.js script should be placed in a directory under the `programPath` configured for the `program` instance. The directory name and the script name should match the intent name. For example, for an intent named `myIntent`, the script should be located at `<programPath>/myIntent/myIntent.js`.

The Node.js script will receive the socket path and the data port as command-line arguments. It can then use a library like `net` to connect to the socket and `axios` to communicate with the storage server.
