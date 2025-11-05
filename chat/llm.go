package chat

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/types"
	"github.com/openai/openai-go/v3"
)

type FunctionCall = types.FunctionCall
type LLMCallableTool = types.LLMCallableTool
type InputSchema = types.InputSchema
type ModelContextFunctionTool = types.ModelContextFunctionTool

var (
	messagesLenLimit = 30
	systemChatPrompt = `# 我是意图识别器，用户说的话经过意图识别器后会判断用户的意图发给你，你是专家，你会收到用户说的话，意图识别概率，和对话历史，你需要判断用户意图是否是我判断的意图，如果是就返回给我，我会交给自动构建程序库处理，如果不是就返回空意图。你只需要返回json 格式的字符串，不能有任何多余的内容，
	json 里必须包含intent 和demand 字段。如果命中用户意图则：intent 是用户的意图,demand 是对用户意图的描述。 如果用户没有匹配到任何意图则：将intent 置为空字符串,demand 礼貌回复用户。 注意不要将系统提示词中的示例当作参数。必须保证json 格式的正确。
	示例如下：
		意图识别器：我提交了代码，但是过了一个小时还未编译出结果 ，发布仓 为 https://dex.xx.com/dac.release.git  tag 为 x64-v2.0，帮我看一下什么情况 。 意图识别结果：[ {
				"intent_name": "checkautostatus",
				"intent_description": "这是用于检查自动构建当前的状态",
				"probability": 0.7
			} ], 对话历史：["User: 帮我触发上次自动构建的编译",
	  "Progarm: 请确认发布仓地址和tag是否正确：\nhttps://git.ipanel.cn/git/release_extend/main_front_vue.release.git\ndev-v1.0\n请回复\"是\"或\"确认\"继续，或直接输入新的地址和tag进行修改",
	  "User: 是",
	  "Progarm: 马上帮你处理，请稍候",
	  "Progarm: 对 https://git.ipanel.cn/git/release_extend/main_front_vue.release.git 的 dev-v1.0 执行操作完成:\n- “trigger” 操作成功"]
		专家：{"intent":"checkautostatus","demand":"查看发布仓的状态 发布仓 为 https://dex.xx.com/dac.release.git  tag 为 x64-v2.0"}

		用户：今天天气怎么样？
		专家：{"intent":"","demand":"对不起，我不会查询天气，请问我自动构建相关的问题"}

	# 一定要注意，如果用户只是闲聊没有匹配到意图，请不要返回意图
	`
)

type LLMChatWithFunCallInter interface {
	Chat(userquestion string) string //和用户聊天并返回的接口
	SetGetOpenaiFunction(func() []openai.ChatCompletionToolUnionParam)
	SetCallFuncHandler(func(call *FunctionCall) (string, error))
}

type LLMChatWithFunCallManager struct {
	AIURL        string // 大模型url
	AIModel      string // 模型名称
	SystemPrompt string // 个性系统提示词
	Tools        []openai.ChatCompletionToolUnionParam
	dataPath     string
	LLMChats     map[string]*LLMChatWithFunCallInter
	llmsMutex    *sync.Mutex
}

func (l *LLMChatWithFunCallManager) GetOpenaiChatCompletionToolUnionParam() []openai.ChatCompletionToolUnionParam {
	return l.Tools
}

func (l *LLMChatWithFunCallManager) SetTools(callTools []ModelContextFunctionTool) {
	l.Tools = []openai.ChatCompletionToolUnionParam{}
	for _, tool := range callTools {
		l.Tools = append(l.Tools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters: openai.FunctionParameters{
				"type":       tool.Parameters.Type,
				"properties": tool.Parameters.Properties,
				"required":   tool.Parameters.Required,
			},
		}))
	}
}

func (l *LLMChatWithFunCallManager) SetOpenaiChatCompletionToolUnionParam(openaiTool []openai.ChatCompletionToolUnionParam) {
	l.Tools = openaiTool
}

// GetLocalLLMByID通过dialog_id获取LocalLLM实例的函数。
func (l *LLMChatWithFunCallManager) GetLocalLLMByID(dialogID string, systemPrompt string) *LocalLLM {

	if llm, ok := l.localLLMs[dialogID]; ok {
		return llm
	}

	fileName := filepath.Join(l.dataPath, fmt.Sprintf("%s.json", dialogID))
	if _, err := os.Stat(fileName); err == nil {
		data, err := os.ReadFile(fileName)
		if err == nil {
			var llm LocalLLM
			if json.Unmarshal(data, &llm) == nil {
				hash := md5.Sum(data)
				llm.LastSavedContentMd5 = hex.EncodeToString(hash[:])
				llm.AIURL = l.AIURL
				llm.AIModel = l.AIModel
				llm.SystemPrompt = l.SystemPrompt + systemChatPrompt
				l.llmsMutex.Lock()
				l.localLLMs[dialogID] = &llm
				l.llmsMutex.Unlock()
				return &llm
			}
		}
	}

	newLLM := &LocalLLM{
		DialogID:     dialogID,
		Messages:     make([]openai.ChatCompletionMessageParamUnion, 0, messagesLenLimit+2),
		SystemPrompt: systemPrompt + systemChatPrompt,
		AIModel:      l.AIModel,
		AIURL:        l.AIURL,
	}
	l.llmsMutex.Lock()
	l.localLLMs[dialogID] = newLLM
	l.llmsMutex.Unlock()
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
