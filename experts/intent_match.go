package experts

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/types"
)

type PossibleIntentions = types.PossibleIntentions
type Attachment = types.Attachment

// 用户的话匹配意图类接口
type IntentMatchInter interface {
	GetIntentName() string                 //获取意图名称
	GetIntentDesc() string                 //获取意图描述
	Matching(string, []Attachment) float64 //用户的话匹配意图系统，返回的第一个参数是匹配的概率 float64
}

// 意图匹配管理器，用于注册、缓存和查找意图匹配器以及意图匹配缓存到文件
type IntentMatchManager struct {
	cacheFilePath     string
	allIntentMatcher  map[string]func() IntentMatchInter
	IntentCache       map[string]string // IntentCache存储用户输入到意图名称的映射。
	cacheMutex        *sync.RWMutex     // cacheMutex保护IntentCache免受并发访问。
	lastSavedCache    string
	vaildMinScore     float64
	messageformatting func(string) string //消息格式化函数，将消息送入意图识别时可以用此函数去处理文字字符串以便更好识别
}

func NewIntentManager() *IntentMatchManager {
	return &IntentMatchManager{
		allIntentMatcher: make(map[string]func() IntentMatchInter),
		IntentCache:      make(map[string]string),
		cacheMutex:       &sync.RWMutex{},
		vaildMinScore:    0.9,
	}
}

func (i *IntentMatchManager) SetCacheFilePath(path string) {
	i.cacheFilePath = path
}

func (i *IntentMatchManager) SetVaildMinScore(score float64) {
	i.vaildMinScore = score
}

// SetMessageFormatFun 设置匹配意图前的消息格式化函数，例如去掉url 等相关内容
func (i *IntentMatchManager) SetMessageFormatFunc(formatting func(string) string) {
	i.messageformatting = formatting
}

func (i *IntentMatchManager) Register(IntentMatcher func() IntentMatchInter, IntentName string) {
	if i.allIntentMatcher[IntentName] != nil {
		return
	}
	i.allIntentMatcher[IntentName] = IntentMatcher
}

func (i *IntentMatchManager) UnRegister(IntentName string) {
	delete(i.allIntentMatcher, IntentName)
}

func (i *IntentMatchManager) GetALLNewIntentMatcher() []IntentMatchInter {
	plugins := make([]IntentMatchInter, 0, len(i.allIntentMatcher))
	for _, ctor := range i.allIntentMatcher {
		plugins = append(plugins, ctor())
	}
	return plugins
}

// LoadIntentCache 从文件系统加载意图缓存到内存。
func (i *IntentMatchManager) LoadIntentCache() {
	i.cacheMutex.Lock()
	defer i.cacheMutex.Unlock()

	data, err := os.ReadFile(i.cacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Intent cache file not found, starting with an empty cache.")
			return
		}
		logger.Errorf("Failed to read Intent cache file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &i.IntentCache); err != nil {
		logger.Errorf("Failed to unmarshal Intent cache data: %v", err)
		return
	}

	// 存储数据的初始状态以避免不必要的保存
	initialData, err := json.MarshalIndent(i.IntentCache, "", "  ")
	if err == nil {
		i.lastSavedCache = string(initialData)
	}

	logger.Infof("Loaded %d Intent cache records.", len(i.IntentCache))
}

// SaveIntentCache 将当前意图缓存保存到文件系统。
func (i *IntentMatchManager) SaveIntentCache() {
	i.cacheMutex.RLock()
	defer i.cacheMutex.RUnlock()

	// Only write to file if the content has actually changed.
	currentData, err := json.MarshalIndent(i.IntentCache, "", "  ")
	if err != nil {
		logger.Errorf("Failed to marshal Intent cache data for saving: %v", err)
		return
	}

	if string(currentData) == i.lastSavedCache {
		return // No changes to save
	}

	if err := os.WriteFile(i.cacheFilePath, currentData, 0644); err != nil {
		logger.Errorf("Failed to write Intent cache file: %v", err)
	} else {
		logger.Info("Saved Intent cache successfully.")
		i.lastSavedCache = string(currentData)
	}
}

// PeriodicCacheSave 定期保存意图缓存。
func (i *IntentMatchManager) PeriodicCacheSave(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug("Periodic Intent cache save check")
		i.SaveIntentCache()
	}
}

// FindBestIntent 首先检查该高速缓存，如果未找到，则执行匹配并缓存结果。 ifsave 参数控制是否缓存新匹配的意图。
func (i *IntentMatchManager) FindBestIntent(relacontent string, attachments []Attachment, ifsave bool) (string, []PossibleIntentions) {
	content := relacontent
	if i.messageformatting != nil {
		content = i.messageformatting(content)
	}

	i.cacheMutex.RLock()
	cachedIntent, found := i.IntentCache[content]
	i.cacheMutex.RUnlock()

	if found {
		logger.Debugf("Cache hit for content. Intent: %s", cachedIntent)
		return cachedIntent, nil
	}

	// 2.如果不在缓存中，则执行匹配
	logger.Debug("Cache miss. Finding best Intent for content.")
	allIntents := i.GetALLNewIntentMatcher()
	if len(allIntents) == 0 {
		logger.Error("No Intents available for matching.")
		return "", nil
	}

	results := make(chan PossibleIntentions, len(allIntents))
	var wg sync.WaitGroup

	for _, exp := range allIntents {
		wg.Add(1)
		go func(e IntentMatchInter) {
			defer wg.Done()
			score := e.Matching(content, attachments)
			results <- PossibleIntentions{IntentName: e.GetIntentName(), Probability: score, IntentDescription: e.GetIntentDesc()}
		}(exp)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	bestIntent := ""
	var maxScore = 0.0

	var possibleIntentions []PossibleIntentions

	for res := range results {
		if res.Probability > maxScore {
			maxScore = res.Probability
			bestIntent = res.IntentName
		}
		possibleIntentions = append(possibleIntentions, res) // 这里可以只放三个最高概率的意图
	}

	logger.Debugf("较高意图概率： %v", possibleIntentions)

	// 3.仅在找到合适的意图时进行缓存
	if maxScore >= i.vaildMinScore {
		logger.Debugf("Found best Intent: %s with score %d. Caching result.", bestIntent, maxScore)
		if ifsave {
			i.CacheContentIntent(content, bestIntent)
		}
		return bestIntent, possibleIntentions
	}

	logger.Debugf("No suitable Intent found with a score >= %d .", i.vaildMinScore)
	return "", possibleIntentions
}

// CacheContentIntent 将内容与意图关联并存储在内存中。
func (i *IntentMatchManager) CacheContentIntent(content string, intent string) {
	i.cacheMutex.Lock()
	i.IntentCache[content] = intent
	i.cacheMutex.Unlock()
}
