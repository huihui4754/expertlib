package experts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	dataPath         = os.Getenv("DATA_PATH")
	allIntentMatcher map[string]func() IntentMatchInter
	IntentCache      = make(map[string]string)                              // IntentCache存储用户输入到专家名称的映射。
	cacheMutex       = &sync.RWMutex{}                                      // cacheMutex保护IntentCache免受并发访问。
	cacheFilePath    = filepath.Join(dataPath, "user", "Intent_cache.json") // cacheFilePath是指向该缓存文件的路径。
	lastSavedCache   string
	urlRegex         = regexp.MustCompile(`(https?://[^\s]+\.release.git)`)
	tagRegex         = regexp.MustCompile(`([a-zA-Z0-9]+-v\d+\.\d+|v\d+\.\d+)`)
	vaildMinScore    = 90
)

type Attachment struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	FileID string `json:"file_id"`
	Option any    `json:"option"`
}

// 用户的话匹配意图类接口
type IntentMatchInter interface {
	GetIntentName() string             //获取意图名称
	GetIntentDesc() string             //获取意图描述
	Matching(string, []Attachment) int //用户的话匹配专家系统，返回的第一个参数是匹配的概率 float64
}

func Register(IntentMatcher func() IntentMatchInter, IntentName string) {
	if allIntentMatcher[IntentName] != nil {
		return
	}
	allIntentMatcher[IntentName] = IntentMatcher
}

func UnRegister(IntentName string) {
	delete(allIntentMatcher, IntentName)
}

func GetALLNewIntentMatcher() []IntentMatchInter {
	plugins := make([]IntentMatchInter, 0, len(allIntentMatcher))
	for _, ctor := range allIntentMatcher {
		plugins = append(plugins, ctor())
	}
	return plugins
}

// LoadIntentCache 从文件系统加载专家缓存。
func LoadIntentCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Intent cache file not found, starting with an empty cache.")
			return
		}
		logger.Errorf("Failed to read Intent cache file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &IntentCache); err != nil {
		logger.Errorf("Failed to unmarshal Intent cache data: %v", err)
		return
	}

	// 存储数据的初始状态以避免不必要的保存
	initialData, err := json.MarshalIndent(IntentCache, "", "  ")
	if err == nil {
		lastSavedCache = string(initialData)
	}

	logger.Infof("Loaded %d Intent cache records.", len(IntentCache))
}

// SaveIntentCache 将当前专家缓存保存到文件系统。
func SaveIntentCache() {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	// Only write to file if the content has actually changed.
	currentData, err := json.MarshalIndent(IntentCache, "", "  ")
	if err != nil {
		logger.Errorf("Failed to marshal Intent cache data for saving: %v", err)
		return
	}

	if string(currentData) == lastSavedCache {
		return // No changes to save
	}

	if err := os.WriteFile(cacheFilePath, currentData, 0644); err != nil {
		logger.Errorf("Failed to write Intent cache file: %v", err)
	} else {
		logger.Info("Saved Intent cache successfully.")
		lastSavedCache = string(currentData)
	}
}

// PeriodicCacheSave 定期保存专家缓存。
func PeriodicCacheSave(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug("Periodic Intent cache save check")
		SaveIntentCache()
	}
}

type PossibleIntentions struct {
	IntentName        string  `json:"intent_name"`
	IntentDescription string  `json:"intent_description"`
	Probability       float64 `json:"probability"`
}

// FindBestIntent 首先检查该高速缓存，如果未找到，则执行匹配并缓存结果。
func FindBestIntent(relacontent string, attachments []Attachment, ifsave bool) (string, []PossibleIntentions) {
	// 1. Check cache first
	content := urlRegex.ReplaceAllString(relacontent, "")
	content = strings.TrimSpace(tagRegex.ReplaceAllString(content, ""))

	cacheMutex.RLock()
	cachedIntent, found := IntentCache[content]
	cacheMutex.RUnlock()

	if found {
		logger.Debugf("Cache hit for content. Intent: %s", cachedIntent)
		return cachedIntent, nil
	}

	// 2.如果不在缓存中，则执行匹配
	logger.Debug("Cache miss. Finding best Intent for content.")
	allIntents := GetALLNewIntentMatcher()
	if len(allIntents) == 0 {
		logger.Error("No Intents available for matching.")
		return "", nil
	}

	type result struct {
		name       string
		score      int
		descprtion string
	}

	results := make(chan result, len(allIntents))
	var wg sync.WaitGroup

	for _, exp := range allIntents {
		wg.Add(1)
		go func(e IntentMatchInter) {
			defer wg.Done()
			score := e.Matching(content, attachments)
			results <- result{name: e.GetIntentName(), score: score, descprtion: e.GetIntentDesc()}
		}(exp)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	bestIntent := ""
	maxScore := -1

	var possibleIntentions []PossibleIntentions

	for res := range results {
		if res.score > maxScore {
			maxScore = res.score
			bestIntent = res.name
		}
		possibleIntentions = append(possibleIntentions, PossibleIntentions{
			IntentName:        res.name,
			IntentDescription: res.descprtion,
			Probability:       float64(res.score) / 100.0,
		})
	}

	logger.Debugf("所有专家概率： %v", possibleIntentions)

	// 3.仅在找到合适的专家时进行缓存
	if maxScore >= vaildMinScore {
		logger.Debugf("Found best Intent: %s with score %d. Caching result.", bestIntent, maxScore)
		if ifsave {
			CacheContentIntent(content, bestIntent)
		}
		return bestIntent, possibleIntentions
	}

	logger.Debugf("No suitable Intent found with a score >= %d .", vaildMinScore)
	return "", possibleIntentions
}

// CacheContentIntent 将内容与意图关联并存储在缓存中。
func CacheContentIntent(content string, intent string) {
	cacheMutex.Lock()
	IntentCache[content] = intent
	cacheMutex.Unlock()
}
