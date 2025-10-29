package experts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/huihui4754/expertlib/types"
	ort "github.com/yalue/onnxruntime_go"
	"github.com/yanyiwu/gojieba"
)

type RNNIntentInit struct {
	libPath         string    // onnxruntime 动态库路径
	libonce         sync.Once // 只加载一次onnxruntime 动态库
	rnnModelPath    string    // rnn模型相关文件目录
	rnnModelIntents map[string]*RNNIntent
	rnnIntentsMutex *sync.RWMutex
}

func (r *RNNIntentInit) SetLibPath(path string) {
	r.libPath = path
}

func (r *RNNIntentInit) SetRNNModelPath(path string) {
	r.rnnModelPath = path
}

// LoadPersistedRemoteIntents 扫描rnnModelPath目录，加载并注册意图
func (r *RNNIntentInit) LoadRNNModelIntents() {
	logger.Info("Scanning for persisted remote intents...")
	files, err := os.ReadDir(r.rnnModelPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Remote intent directory does not exist, skipping scan.")
			return
		}
		logger.Errorf("Failed to read remote intent directory '%s': %v", r.rnnModelPath, err)
		return
	}

	count := 0
	for _, file := range files {
		if file.IsDir() {
			intentName := file.Name()
			intentDir := filepath.Join(r.rnnModelPath, intentName)
			logger.Infof("Found persisted intent: %s. Attempting to reload.", intentName)

			// Read description from README.md if it exists
			var description string
			readmePath := filepath.Join(intentDir, "README.md")
			if readmeBytes, err := os.ReadFile(readmePath); err == nil {
				description = string(readmeBytes)
			}

			// Read weight from weight.json
			var weight float32 = 1.0 // Default weight
			weightPath := filepath.Join(intentDir, "weight.json")
			if weightBytes, err := os.ReadFile(weightPath); err == nil {
				var weightData map[string]float32
				if json.Unmarshal(weightBytes, &weightData) == nil {
					if w, ok := weightData["weight"]; ok {
						weight = w
					}
				}
			}

			// Use the existing registration logic to load the intent
			if err := r.LoadRNNIntent(intentName, description, weight); err != nil {
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

type ExpertToProgramMessage = types.ExpertToProgramMessage

// once 保证初始化代码只执行一次

// InitializeONNX 负责设置共享库路径。
// 无论此函数被调用多少次，实际的设置操作都只会执行一次。
func (r *RNNIntentInit) InitializeONNX() {
	r.libonce.Do(func() {
		// 在这里放置你的 .so 文件路径
		// 你可以从环境变量、配置文件或固定路径读取
		ort.SetSharedLibraryPath(r.libPath)
		err := ort.InitializeEnvironment()
		if err != nil {
			logger.Errorf("Error initializing ONNX Runtime: %v", err)
		}
	})
}

// LoadRNNIntent 创建、加载并注册一个新的rnn 意图识别
func (r *RNNIntentInit) LoadRNNIntent(name, description string, weight float32) error {
	r.rnnIntentsMutex.Lock()
	defer r.rnnIntentsMutex.Unlock()

	if _, exists := r.rnnModelIntents[name]; exists {
		return fmt.Errorf("remote intent with name '%s' already exists", name)
	}

	newIntent := &RNNIntent{
		intentName: name,
		intentDesc: description,
		Weight:     weight,
	}

	// LoadModel 会处理 ONNX/Jieba 初始化和向 expert/core 的注册
	if err := newIntent.LoadModel(); err != nil {
		logger.Errorf("加载远程意图 '%s' 失败: %v", name, err)
		newIntent.Close()
		return err
	}

	r.rnnModelIntents[name] = newIntent
	logger.Infof("成功加载并注册了新的远程意图: %s", name)
	return nil
}

// UnloadRNNIntent 清理rnn 意图识别实例
func (r *RNNIntentInit) UnloadRNNIntent(intentName string) error {
	r.rnnIntentsMutex.Lock()
	defer r.rnnIntentsMutex.Unlock()

	// 1.查找Intent实例
	remoteIntent, ok := r.rnnModelIntents[intentName]
	if !ok {
		return fmt.Errorf("remote intent '%s' not found or not active", intentName)
	}

	// 5.释放Intent实例持有的ONNX和其他资源
	remoteIntent.Close()

	// 6.从活动意图映射中删除
	delete(r.rnnModelIntents, intentName)

	logger.Infof("成功注销并清理了远程意图: %s", intentName)
	return nil
}

// UnloadRNNIntent 清理所有的 rnn 意图识别实例
func (r *RNNIntentInit) UnloadALLRNNIntent(intentName string) error {
	r.rnnIntentsMutex.Lock()
	defer r.rnnIntentsMutex.Unlock()

	for _, remoteIntent := range r.rnnModelIntents {
		// 5.释放Intent实例持有的ONNX和其他资源
		remoteIntent.Close()
	}

	r.rnnModelIntents = make(map[string]*RNNIntent)

	logger.Infof("成功注销并清理所有了远程意图: %s", intentName)
	return nil
}

func (r *RNNIntentInit) GetAllRNNIntents() map[string]*RNNIntent {
	return r.rnnModelIntents
}

type RNNIntent struct {
	modelDataPath string
	jieba         *gojieba.Jieba
	vocab         map[string]int64
	session       *ort.DynamicAdvancedSession
	Weight        float32
	intentName    string
	intentDesc    string
}

func (r *RNNIntent) SetModelDataPath(data_path string) {
	r.modelDataPath = data_path
}

// Close释放与RNNIntent关联的资源。
func (r *RNNIntent) Close() {
	if r.jieba != nil {
		r.jieba.Free()
	}
	if r.session != nil {
		r.session.Destroy()
	}
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

func (r *RNNIntent) Matching(content string, attachments []Attachment) float64 {

	indices := TextToIndices(content, r.vocab, r.jieba)
	if len(indices) == 0 {
		logger.Errorf("Skipping empty input for text: '%s'", content)
		return 0.0
	}

	// 使用正确的形状和类型创建输入张量（int64）
	inputShape := ort.NewShape(1, int64(len(indices)))
	inputTensor, err := ort.NewTensor(inputShape, indices)
	if err != nil {
		logger.Errorf("Failed to create input tensor for text '%s': %v", content, err)
		return 0.0
	}

	// 创建具有固定形状的空输出张量。
	outputShape := ort.NewShape(1, 2)
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		logger.Errorf("Failed to create output tensor for text '%s': %v", content, err)
		return 0.0
	}

	// 清理本次迭代的张量
	defer inputTensor.Destroy()
	defer outputTensor.Destroy()

	// 通过将张量传递给Run方法来运行推理。
	err = r.session.Run([]ort.Value{inputTensor}, []ort.Value{outputTensor})
	if err != nil {
		logger.Errorf("Inference failed for text '%s': %v", content, err)
	} else {
		// Get output probabilities
		probabilities := outputTensor.GetData()
		positiveProbability := probabilities[1] // Assuming index 1 is the 'positive' class

		logger.Infof("<<<<<<<<<  Text: %s Probability: %.4f", content, positiveProbability)

		// --- 4. 业务逻辑 ---
		if (positiveProbability * r.Weight) > 0.90 {
			logger.Debug("意图匹配成功")
			return float64(positiveProbability)
		} else {
			logger.Debug("意图: 其他")
			return float64(positiveProbability)
		}
	}

	return 0.0
}

func (r *RNNIntent) GetIntentName() string {
	return r.intentName
}

func (r *RNNIntent) GetIntentDesc() string {
	return r.intentDesc
}

func (r *RNNIntent) NewCheckAutoStatusExpertMatch() IntentMatchInter {
	return r
}

func (r *RNNIntent) LoadModel() error {

	r.jieba = gojieba.NewJieba()

	vocabFilePath := filepath.Join(r.modelDataPath, r.intentName, "vocab_rnn.json")
	vocabFile, err := os.Open(vocabFilePath)
	if err != nil {
		return fmt.Errorf("failed to open vocab file '%s': %w", vocabFilePath, err)
	}
	defer vocabFile.Close()

	if err := json.NewDecoder(vocabFile).Decode(&(r.vocab)); err != nil {
		return fmt.Errorf("failed to decode vocab file: %w", err)
	}

	// --- 2. Create a Dynamic AdvancedSession ---
	// Use DynamicAdvancedSession for models with dynamic input shapes.
	modelPath := filepath.Join(r.modelDataPath, r.intentName, "model_rnn.onnx")
	r.session, err = ort.NewDynamicAdvancedSession(modelPath,
		[]string{"input"}, []string{"output"}, nil)
	if err != nil {
		return fmt.Errorf("failed to create dynamic session for model '%s': %w", modelPath, err)
	}

	logger.Infof("rnn意图识别 %s -- 注册成功", r.intentName)
	return nil
}
