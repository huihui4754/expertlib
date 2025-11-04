package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/types"
)

var (
	dataPath         = os.Getenv("DATA_PATH")
	dialogPath       = filepath.Join(dataPath, "dialogs")
	messagesLenLimit = 30
	localLLMs        = make(map[string]*LocalLLM)
	llmsMutex        sync.Mutex

	// systemPrompt = `# 我是意图识别器，用户说的话经过意图识别器后会判断用户的意图发给你，你是自动构建专家，你会收到用户说的话，意图识别概率，和对话历史，你需要判断用户意图是否是我判断的意图，如果是就返回给我，我会交给自动构建程序库处理，如果不是就返回空意图。你只需要返回json 格式的字符串，不能有任何多余的内容，
	// json 里必须包含intent 和demand 字段，intent 是用户的意图，demand 是对用户意图的描述，如果用户没有匹配到任何意图，请将intent 置为空字符串,demand 永远不能为空，要保持礼貌回复用户。自动构建程序库都会需要发布仓的地址和发布tag ,如果用户没有提供发布仓地址和tag  就不需要带上该参数,注意不要将系统提示词中的示例当作参数。必须保证json 格式的正确。
	// 示例如下：
	// 	意图识别器：我提交了代码，但是过了一个小时还未编译出结果 ，发布仓 为 https://dex.xx.com/dac.release.git  tag 为 x64-v2.0，帮我看一下什么情况 。 意图识别结果：[ {
	// 			"intent_name": "checkautostatus",
	// 			"intent_description": "这是用于检查自动构建当前的状态",
	// 			"probability": 0.7
	// 		} ], 对话历史：["User: 帮我触发上次自动构建的编译",
	//   "Progarm: 请确认发布仓地址和tag是否正确：\nhttps://git.ipanel.cn/git/release_extend/main_front_vue.release.git\ndev-v1.0\n请回复\"是\"或\"确认\"继续，或直接输入新的地址和tag进行修改",
	//   "User: 是",
	//   "Progarm: 马上帮你处理，请稍候",
	//   "Progarm: 对 https://git.ipanel.cn/git/release_extend/main_front_vue.release.git 的 dev-v1.0 执行操作完成:\n- “trigger” 操作成功"]
	// 	专家：{"intent":"checkautostatus","demand":"查看发布仓的状态 发布仓 为 https://dex.xx.com/dac.release.git  tag 为 x64-v2.0"}

	// 	用户：今天天气怎么样？
	// 	专家：{"intent":"","demand":"对不起，我不会查询天气，请问我自动构建相关的问题"}

	// # 一定要注意，如果用户只是闲聊没有匹配到意图，请不要返回意图
	// `
)

type ChatAIMessage = types.ChatAIMessage

type LocalLLM struct {
	DialogID         string            `json:"dialog_id"`
	AIURL            string            `json:"-"`
	AIModel          string            `json:"-"`
	SystemPrompt     string            `json:"-"`
	Messages         []ChatAIMessage   `json:"messages"`
	ExtraBody        map[string]any    `json:"-"`
	ExtraHeader      map[string]string `json:"-"`
	lastSavedContent []byte            `json:"-"`
	Relpying         bool              `json:"-"`
}

// RequestLLM 非流式接口
func (c *LocalLLM) RequestLLM(messages []ChatAIMessage) (string, error) {
	if c.Relpying {
		return "", fmt.Errorf("当前对话正在进行中，请稍后再试")
	}
	c.Relpying = true
	defer func() { c.Relpying = false }()

	if c.SystemPrompt != "" && len(c.Messages) == 0 {
		c.Messages = append(c.Messages, ChatAIMessage{
			Role:    "system",
			Content: c.SystemPrompt,
		})
	}
	// 将原始消息追加到新切片
	c.Messages = append(c.Messages, messages...)

	if len(c.Messages) > messagesLenLimit {
		c.Messages = append(c.Messages[:0], c.Messages[2:]...)
	}

	requestBody := map[string]any{
		"model":    c.AIModel,
		"messages": c.Messages,
	}
	maps.Copy(requestBody, c.ExtraBody)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("error: marshal request error: %v", err)
	}

	if c.AIURL == "" {
		return "", fmt.Errorf("error: AI URL is empty")
	}

	req, err := http.NewRequest("POST", c.AIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error: create request error: %v", err)
	}

	for k, v := range c.ExtraHeader {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error: HTTP request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error: AI request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error: read response error: %v", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("error: parse response error: %v", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("error: empty choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// GetLocalLLMByID a function to get a LocalLLM instance by dialog_id.
func GetLocalLLMByID(dialogID string) *LocalLLM {

	if llm, ok := localLLMs[dialogID]; ok {
		return llm
	}

	fileName := filepath.Join(dialogPath, fmt.Sprintf("%s.json", dialogID))
	if _, err := os.Stat(fileName); err == nil {
		data, err := os.ReadFile(fileName)
		if err == nil {
			var llm LocalLLM
			if json.Unmarshal(data, &llm) == nil {
				llm.lastSavedContent = data
				llmsMutex.Lock()
				localLLMs[dialogID] = &llm
				llmsMutex.Unlock()
				return &llm
			}
		}
	}

	newLLM := &LocalLLM{
		DialogID:     dialogID,
		Messages:     make([]ChatAIMessage, 0, messagesLenLimit+2),
		SystemPrompt: systemPrompt,
		// AIURL:        "http://192.168.101.130:8008/v1/chat/completions",
		// AIModel:      "/data/Qwen3-0.6B",
		// AIURL:   "http://192.168.101.130:8010/v1/chat/completions",
		// AIModel: "Qwen3-32B-AWQ",
		AIModel: "Qwen3-8B-AWQ",
		AIURL:   "http://192.168.101.130:8011/v1/chat/completions",
		ExtraBody: map[string]any{
			"chat_template_kwargs": map[string]any{
				"enable_thinking": false,
			},
			"response_format": map[string]any{
				"type": "json_object",
			},
		},
	}
	llmsMutex.Lock()
	localLLMs[dialogID] = newLLM
	llmsMutex.Unlock()
	return newLLM
}

func saveAllDialogs() {
	llmsMutex.Lock()
	defer llmsMutex.Unlock()

	for id, llm := range localLLMs {
		data, err := json.MarshalIndent(llm, "", "  ")
		if err != nil {
			logger.Errorf("Failed to marshal dialog %s: %v", id, err)
			continue
		}

		if bytes.Equal(data, llm.lastSavedContent) {
			continue
		}

		fileName := filepath.Join(dialogPath, fmt.Sprintf("%s.json", id))
		if err := os.WriteFile(fileName, data, 0644); err != nil {
			logger.Errorf("Failed to write dialog file %s: %v", fileName, err)
		} else {
			llm.lastSavedContent = data
		}
	}
}

func init() {
	// 确保用户对话数据的目录存在
	if err := os.MkdirAll(dialogPath, 0755); err != nil {
		logger.Fatalf("Failed to create user data directory: %v", err)
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			saveAllDialogs()
		}
	}()
}
