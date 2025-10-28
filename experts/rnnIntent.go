package experts

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gorilla/websocket"
	ort "github.com/yalue/onnxruntime_go"
	"github.com/yanyiwu/gojieba"
)

var (
	libonce            sync.Once
	autoBuildToolConn  *websocket.Conn
	channelMap         = map[string]IntentMessageChan{}
	programPath        = os.Getenv("PROGRAM_PATH")
	intentsProgramPath = filepath.Join(programPath, "intents")

	// Callback function to handle messages from experts, set by main.
	expertMessageHandler func(message []byte)
	// Map to track active remote intents for resource cleanup.
	activeRemoteIntents = make(map[string]*RemoteIntent)
	// Mutex to protect access to the activeRemoteIntents map.
	remoteIntentsMutex = &sync.RWMutex{}
)

// SetExpertMessageHandler allows the main package to inject its message handling logic.
func SetExpertMessageHandler(handler func(message []byte)) {
	expertMessageHandler = handler
}

// LoadPersistedRemoteIntents scans the registration directory and reloads any previously registered intents.
func LoadPersistedRemoteIntents() {
	logger.Info("Scanning for persisted remote intents...")
	files, err := ioutil.ReadDir(registerModelDataPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Remote intent directory does not exist, skipping scan.")
			return
		}
		logger.Errorf("Failed to read remote intent directory '%s': %v", registerModelDataPath, err)
		return
	}

	count := 0
	for _, file := range files {
		if file.IsDir() {
			intentName := file.Name()
			intentDir := filepath.Join(registerModelDataPath, intentName)
			logger.Infof("Found persisted intent: %s. Attempting to reload.", intentName)

			// Read description from README.md if it exists
			var description string
			readmePath := filepath.Join(intentDir, "README.md")
			if readmeBytes, err := ioutil.ReadFile(readmePath); err == nil {
				description = string(readmeBytes)
			}

			// Read weight from weight.json
			var weight float32 = 1.0 // Default weight
			weightPath := filepath.Join(intentDir, "weight.json")
			if weightBytes, err := ioutil.ReadFile(weightPath); err == nil {
				var weightData map[string]float32
				if json.Unmarshal(weightBytes, &weightData) == nil {
					if w, ok := weightData["weight"]; ok {
						weight = w
					}
				}
			}

			// Use the existing registration logic to load the intent
			if err := LoadAndRegisterNewRemoteIntent(intentName, description, weight); err != nil {
				logger.Errorf("Failed to reload persisted intent '%s': %v", intentName, err)
			} else {
				count++
			}
		}
	}
	if count > 0 {
		logger.Infof("Successfully reloaded %d persisted remote intent(s).", count)
	}
}

// StartListening starts a goroutine to listen on an expert's output channel.
func StartListening(expertName string, channelOut <-chan []byte) {
	if expertMessageHandler == nil {
		logger.Errorf("Message handler not set for intentexpert. Cannot start listener for %s.", expertName)
		return
	}
	logger.Debugf("开始监听动态专家 %s 的通道", expertName)
	go func() {
		for message := range channelOut {
			expertMessageHandler(message)
		}
		logger.Infof("动态专家 %s 的通道已关闭，监听结束。", expertName)
	}()
}

type IntentMessageChan struct {
	ChannelIn  chan<- []byte // 这里的 ChannelIn 是发送给专家程序库websocket，故对应着意图中的out
	ChannelOut <-chan []byte // 这里的 ChannelOut 是从专家程序库websocket接收，故对应着意图中的in
}

type Attachment = expert.Attachment

type ExpertToolMessage struct {
	EventType int    `json:"event_type"`
	DialogID  string `json:"dialog_id"`
	MessageID string `json:"message_id"`
	Intention string `json:"intention"` // Intention 是专家内部用于告诉程序库匹配的意图加上的
	Messages  struct {
		Content     string       `json:"content"`
		Attachments []Attachment `json:"attachments"`
	} `json:"messages"`
}

func Register(IntentName string, expertChanIn chan<- []byte, expertChanOut <-chan []byte) {
	channelMap[IntentName] = IntentMessageChan{
		ChannelIn:  expertChanIn,
		ChannelOut: expertChanOut,
	}
	go func(chanName string, c <-chan []byte) {
		for val := range c {
			logger.Debug("发送消息到程序库: %v", string(val))
			// 将接收到的值发送出去
			err := (*autoBuildToolConn).WriteMessage(websocket.TextMessage, val)
			if err != nil {
				logger.Error("发送消息失败:", err)
				continue
			}
		}
		logger.Debug("channel %s closed\n", chanName)
	}(IntentName, expertChanOut)

}

func UnRegister(expertName string) {
	delete(channelMap, expertName)
}

// once 保证初始化代码只执行一次

// InitializeONNX 负责设置共享库路径。
// 无论此函数被调用多少次，实际的设置操作都只会执行一次。
func InitializeONNX() {
	libonce.Do(func() {
		// 在这里放置你的 .so 文件路径
		// 你可以从环境变量、配置文件或固定路径读取
		ort.SetSharedLibraryPath(filepath.Join(intentsProgramPath, "libonnxruntime.so.1.22.0"))
		err := ort.InitializeEnvironment()
		if err != nil {
			logger.Errorf("Error initializing ONNX Runtime: %v", err)
		}
	})
}

// textToIndices根据词汇表将文本转换为int64索引片段。
func TextToIndices(text string, vocab map[string]int64, jieba *gojieba.Jieba) []int64 {
	// Tokenize using jieba
	tokens := jieba.Cut(text, true)

	// Convert tokens to indices
	indices := make([]int64, 0, len(tokens))
	for _, token := range tokens {
		if index, found := vocab[token]; found {
			indices = append(indices, index)
		} else {
			// Use the index for <UNK> token if word is not in vocab
			indices = append(indices, vocab["<UNK>"])
		}
	}
	return indices
}

// LoadAndRegisterNewRemoteIntent 创建、加载并注册一个新的远程意图
func LoadAndRegisterNewRemoteIntent(name, description string, weight float32) error {
	remoteIntentsMutex.Lock()
	defer remoteIntentsMutex.Unlock()

	if _, exists := activeRemoteIntents[name]; exists {
		return fmt.Errorf("remote intent with name '%s' already exists", name)
	}

	newIntent := &RemoteIntent{
		ChannelIn:  make(chan []byte, 10),
		ChannelOut: make(chan []byte, 10),
		intentName: name,
		intentDesc: description,
		Weight:     weight,
	}

	// LoadModel 会处理 ONNX/Jieba 初始化和向 expert/core 的注册
	if err := newIntent.LoadModel(); err != nil {
		logger.Errorf("加载远程意图 '%s' 失败: %v", name, err)
		return err
	}

	activeRemoteIntents[name] = newIntent
	logger.Infof("成功加载并注册了新的远程意图: %s", name)
	return nil
}

// sendJSONResponse 构造并发送一个标准的JSON响应
func sendJSONResponse(w http.ResponseWriter, code int, info string, httpStatus int) {
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		Code int    `json:"code"`
		Info string `json:"info"`
	}{
		Code: code,
		Info: info,
	}
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// 如果JSON编码失败，记录错误，但此时可能无法再向客户端发送响应
		logger.Errorf("Failed to encode JSON response: %v", err)
	}
}

// RegisterRemoteIntentHandler 处理来自远程服务的意图注册请求
func RegisterRemoteIntentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONResponse(w, 1, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendJSONResponse(w, 1, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req RemoteIntentRegistrationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSONResponse(w, 1, "Failed to parse request JSON", http.StatusBadRequest)
		return
	}

	if req.IntentName == "" || req.OnnxModelData == "" || req.VocabJsonData == "" {
		sendJSONResponse(w, 1, "Missing required fields: intent_name, onnx_model_data, vocab_json_data", http.StatusBadRequest)
		return
	}

	// 1. 解码 Base64 数据
	onnxBytes, err := base64.StdEncoding.DecodeString(req.OnnxModelData)
	if err != nil {
		sendJSONResponse(w, 1, "Invalid base64 for onnx_model_data", http.StatusBadRequest)
		return
	}

	vocabBytes, err := base64.StdEncoding.DecodeString(req.VocabJsonData)
	if err != nil {
		sendJSONResponse(w, 1, "Invalid base64 for vocab_json_data", http.StatusBadRequest)
		return
	}

	// 2. 创建意图目录并保存文件
	intentDir := filepath.Join(registerModelDataPath, req.IntentName)
	if err := os.MkdirAll(intentDir, 0755); err != nil {
		sendJSONResponse(w, 1, "Failed to create intent directory", http.StatusInternalServerError)
		return
	}

	if err := ioutil.WriteFile(filepath.Join(intentDir, "model_rnn.onnx"), onnxBytes, 0644); err != nil {
		sendJSONResponse(w, 1, "Failed to save onnx model", http.StatusInternalServerError)
		return
	}

	if err := ioutil.WriteFile(filepath.Join(intentDir, "vocab_rnn.json"), vocabBytes, 0644); err != nil {
		sendJSONResponse(w, 1, "Failed to save vocab json", http.StatusInternalServerError)
		return
	}

	// (可选但推荐) 保存描述信息
	if req.IntentDescription != "" {
		_ = ioutil.WriteFile(filepath.Join(intentDir, "README.md"), []byte(req.IntentDescription), 0644)
	}

	// 保存权重文件
	weight := req.Weight
	if weight == 0 {
		weight = 1.0 // Default to 1.0 if not provided
	}
	weightData := map[string]float32{"weight": weight}
	weightBytes, err := json.Marshal(weightData)
	if err != nil {
		sendJSONResponse(w, 1, "Failed to marshal weight data", http.StatusInternalServerError)
		return
	}
	if err := ioutil.WriteFile(filepath.Join(intentDir, "weight.json"), weightBytes, 0644); err != nil {
		sendJSONResponse(w, 1, "Failed to save weight file", http.StatusInternalServerError)
		return
	}

	// 3. 动态加载并注册新的意图
	if err := LoadAndRegisterNewRemoteIntent(req.IntentName, req.IntentDescription, weight); err != nil {
		sendJSONResponse(w, 1, fmt.Sprintf("Failed to load and register new intent: %v", err), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, 0, fmt.Sprintf("Intent '%s' registered successfully.", req.IntentName), http.StatusOK)
	logger.Infof("远程意图 '%s' 注册并加载成功。", req.IntentName)
}

// UnregisterIntent handles the complete cleanup of a remote intent.
func UnregisterIntent(intentName string) error {
	remoteIntentsMutex.Lock()
	defer remoteIntentsMutex.Unlock()

	// 1. Find the intent instance
	remoteIntent, ok := activeRemoteIntents[intentName]
	if !ok {
		return fmt.Errorf("remote intent '%s' not found or not active", intentName)
	}

	// 2. Close the channels to terminate goroutines gracefully
	// Closing ChannelOut will stop the listener started by StartListening
	close(remoteIntent.ChannelOut)
	// Closing ChannelIn will prevent any more messages from being sent to it
	close(remoteIntent.ChannelIn)

	// 3. Unregister from the central expert registry (assuming this function exists)
	expert.UnRegister(remoteIntent.NewCheckAutoStatusExpertMatch, intentName)

	// 4. Unregister from the local channel map
	UnRegister(intentName)

	// 5. Release ONNX and other resources held by the intent instance
	remoteIntent.Close()

	// 6. Delete from the active intents map
	delete(activeRemoteIntents, intentName)

	// 7. Delete model files from disk
	intentDir := filepath.Join(registerModelDataPath, intentName)
	if err := os.RemoveAll(intentDir); err != nil {
		logger.Errorf("Failed to delete intent directory '%s': %v", intentDir, err)
		// Continue with cleanup even if file deletion fails
	}

	logger.Infof("成功注销并清理了远程意图: %s", intentName)
	return nil
}

// UnregisterRemoteIntentHandler handles requests to unregister and delete a remote intent.
func UnregisterRemoteIntentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONResponse(w, 1, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IntentName string `json:"intent_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONResponse(w, 1, "Failed to parse request JSON", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.IntentName == "" {
		sendJSONResponse(w, 1, "Missing required field: intent_name", http.StatusBadRequest)
		return
	}

	if err := UnregisterIntent(req.IntentName); err != nil {
		sendJSONResponse(w, 1, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, 0, fmt.Sprintf("Intent '%s' unregistered successfully.", req.IntentName), http.StatusOK)
}

func init() {
	InitializeONNX()
	logger.Info("自动构建工具链接成功")
}
