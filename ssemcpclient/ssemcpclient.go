package ssemcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
	"github.com/r3labs/sse/v2"
)

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

func SetLogger(level loglevel.Level) {
	logger.SetLevel(level)
}

// MCPSSEClient 表示SSE长连接客户端
type MCPSSEClient struct {
	url              string
	endpointURL      string
	initCallback     func(*MCPSSEClient)
	errorCallback    func(*MCPSSEClient)
	isInitialized    bool
	messageEvents    []map[string]any
	messageID        int
	client           *http.Client
	tools            []any
	error            bool
	connection       *sse.Client
	serverInfo       map[string]any
	instructions     any
	mu               sync.Mutex //结构体对象保持关键数据同步的锁
	initChan         chan struct{}
	responseChannels map[int]chan string
	eventChan        chan *sse.Event
	ReconnectChan    chan struct{}
	stopChan         chan struct{}
}

// NewSSEClient 创建新的SSE客户端
func NewSSEClient(url string, initCallback, errorCallback func(*MCPSSEClient)) *MCPSSEClient {
	return &MCPSSEClient{
		url:           url,
		initCallback:  initCallback,
		errorCallback: errorCallback,
		messageEvents: make([]map[string]interface{}, 0),
		messageID:     0,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		tools:            nil,
		initChan:         make(chan struct{}),
		responseChannels: make(map[int]chan string, 10),
		eventChan:        make(chan *sse.Event),
		ReconnectChan:    make(chan struct{}),
		stopChan:         make(chan struct{}),
	}
}

// Init 初始化SSE连接并完成端点发现和初始化流程
func (c *MCPSSEClient) Init() bool {
	if c.isInitialized {
		logger.Debug("SSE client already initialized")
		return true
	}
	logger.Debug("Initializing SSE client")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 启动连接和处理
	go c.connectAndProcess()

	// 等待初始化完成或超时
	select {
	case <-c.initChan:
		logger.Debug("SSE client initialization completed")
		c.isInitialized = true
		if c.initCallback != nil {
			c.initCallback(c)
		}
		return true
	case <-ctx.Done():
		logger.Debug("SSE client initialization timed out")
		c.mu.Lock()
		c.error = true
		c.mu.Unlock()
		if c.errorCallback != nil {
			c.errorCallback(c)
		}
		return false
	}
}

func (c *MCPSSEClient) connectAndProcess() {
	logger.Debug("Attempting to connect to SSE server")

	c.connection = sse.NewClient(c.url)

	c.connection.Headers = map[string]string{
		"Accept":        "text/event-stream",
		"Cache-Control": "no-cache",
	}
	c.connection.OnDisconnect(func(x *sse.Client) {
		logger.Warn("SSE client disconnected")
		close(c.ReconnectChan)
	})

	err := c.connection.SubscribeChan("", c.eventChan)
	if err != nil {
		logger.Errorf("SSE connection error: %v\n", err)
		c.mu.Lock()
		c.error = true
		c.mu.Unlock()
		if c.errorCallback != nil {
			c.errorCallback(c)
		}
	}

	go func() {
		for {
			select {
			case event := <-c.eventChan:
				c.processEvent(event)
			case <-c.stopChan:
				logger.Debug("Stopping event processing goroutine")
				return
			}
		}
	}()
}

func (c *MCPSSEClient) processEvent(event *sse.Event) {
	logger.Debugf("Received SSE event: %s, data: %s\n", event.Event, string(event.Data))

	switch string(event.Event) {
	case "endpoint":
		c.handleEndpointEvent(string(event.Data))
	case "message":
		c.handleMessageEvent(string(event.Data))
	default:
		logger.Debugf("Unknown SSE event: %s\n", event.Event)
	}
}

func (c *MCPSSEClient) handleEndpointEvent(data string) {
	endpointURL := joinURL(c.url, data)
	logger.Debugf("Received endpoint URL: %s\n", endpointURL)

	baseParsed, err1 := url.Parse(c.url)
	endpointParsed, err2 := url.Parse(endpointURL)

	if err1 != nil || err2 != nil {
		logger.Debugf("Error parsing URLs: %v, %v\n", err1, err2)
		return
	}

	if baseParsed.Host != endpointParsed.Host || baseParsed.Scheme != endpointParsed.Scheme {
		errMsg := fmt.Sprintf("Endpoint origin does not match connection origin: %s", endpointURL)
		logger.Debug(errMsg)
		return
	}

	baseQuery := baseParsed.Query()

	// 将基础URL的查询参数合并到目标URL的查询参数中
	// 注意：如果存在同名参数，baseQuery 的值会覆盖 endpointQuery 中的值
	endpointQuery := endpointParsed.Query()
	for key, values := range baseQuery {
		for _, value := range values {
			endpointQuery.Set(key, value) // 使用 Set 确保参数被覆盖而不是追加
		}
	}
	endpointParsed.RawQuery = endpointQuery.Encode()
	updatedEndpointURL := endpointParsed.String()

	c.mu.Lock()
	c.endpointURL = updatedEndpointURL
	logger.Debugf("c.endpointURL: %s\n", c.endpointURL)
	c.mu.Unlock()

	// 完成初始化流程
	go c.initializeEndpoint()
}

func (c *MCPSSEClient) handleMessageEvent(data string) {
	var msgData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &msgData); err != nil {
		logger.Debugf("Error parsing message event data: %v\n", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if id, ok := msgData["id"].(float64); ok {
		idInt := int(id)
		if ch, exists := c.responseChannels[idInt]; exists {
			ch <- data
			close(ch)
		}

		switch idInt {
		case 0:
			if instructions, ok := msgData["instructions"]; ok {
				c.instructions = instructions
				logger.Debug("Received instructions\n")
			}
			if serverInfo, ok := msgData["serverInfo"]; ok {
				c.serverInfo = serverInfo.(map[string]interface{})
				logger.Debug("Received serverInfo\n")
			}
		case 1:
			if result, ok := msgData["result"].(map[string]interface{}); ok {
				if tools, ok := result["tools"]; ok {
					c.tools = tools.([]interface{})
					logger.Debug("Received tools\n")
				}
			}
		}
	}

	if len(c.messageEvents) > 50 {
		c.messageEvents = c.messageEvents[1:]
	}
	c.messageEvents = append(c.messageEvents, msgData)
}

func (c *MCPSSEClient) initializeEndpoint() {
	if !c.sendInitializationRequest() {
		logger.Debug("Initialization request failed")
		return
	}
	time.Sleep(100 * time.Millisecond)

	if !c.sendInitializedNotification() {
		logger.Debug("Initialization notification failed")
		return
	}
	time.Sleep(100 * time.Millisecond)

	if !c.initGetCallableTools() {
		logger.Debug("Initialization get_callable_tools failed")
		return
	}
	// time.Sleep(100 * time.Millisecond)

	close(c.initChan)
}

func (c *MCPSSEClient) sendInitializationRequest() bool {
	c.mu.Lock()
	id := c.messageID
	c.mu.Unlock()

	initData := map[string]interface{}{
		"method": "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities": map[string]interface{}{
				"sampling": map[string]interface{}{},
				"roots": map[string]interface{}{
					"listChanged": false,
				},
			},
			"clientInfo": map[string]interface{}{
				"name":    "mcp",
				"version": "0.1.0",
			},
		},
		"jsonrpc": "2.0",
		"id":      id,
	}
	_, err := c.postJSON(c.endpointURL, initData)
	return err == nil
}

func (c *MCPSSEClient) initGetCallableTools() bool {
	c.mu.Lock()
	c.messageID++
	id := c.messageID
	c.mu.Unlock()

	data := map[string]interface{}{
		"method":  "tools/list",
		"params":  map[string]interface{}{},
		"jsonrpc": "2.0",
		"id":      id,
	}
	_, err := c.postJSON(c.endpointURL, data)
	return err == nil
}

func (c *MCPSSEClient) sendInitializedNotification() bool {
	data := map[string]interface{}{
		"method":  "notifications/initialized",
		"jsonrpc": "2.0",
	}
	_, err := c.postJSON(c.endpointURL, data)
	return err == nil
}

func (c *MCPSSEClient) postJSON(url string, data map[string]interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String(), nil
}

func (c *MCPSSEClient) Close() {
	close(c.stopChan)
	if c.connection != nil {
		c.connection.Unsubscribe(c.eventChan)
	}
	c.client = nil

	c.mu.Lock()
	for id, ch := range c.responseChannels {
		close(ch)
		delete(c.responseChannels, id)
	}
	c.mu.Unlock()

	logger.Debug("SSE connection closed")
}

func (c *MCPSSEClient) CallFunction(functionName string, arguments map[string]interface{}) (bool, string, error) {
	c.mu.Lock()
	c.messageID++
	if c.messageID == 10000 {
		c.messageID = 3
	}
	id := c.messageID
	respChan := make(chan string, 1)
	c.responseChannels[id] = respChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.responseChannels, id)
		c.mu.Unlock()
	}()

	callData := map[string]interface{}{
		"method": "tools/call",
		"params": map[string]interface{}{
			"name":      functionName,
			"arguments": arguments,
		},
		"jsonrpc": "2.0",
		"id":      id,
	}

	_, err := c.postJSON(c.endpointURL, callData)
	if err != nil {
		close(c.ReconnectChan)
		return false, "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		return false, "", errors.New("timeout waiting for function call result")
	case result, ok := <-respChan:
		if !ok {
			return false, "", errors.New("response channel closed unexpectedly")
		}
		return true, result, nil
	}
}

func joinURL(base, path string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return path
	}
	relativeURL, err := url.Parse(path)
	if err != nil {
		return path
	}
	return baseURL.ResolveReference(relativeURL).String()
}

func errorCallback(client *MCPSSEClient) {
	logger.Debug("SSE client error callback triggered. The main routine will handle reconnection.")
	client.Close()
}

func RunRoutine(ctx context.Context, url string, sseClient *atomic.Pointer[MCPSSEClient], initCallback func(*MCPSSEClient)) {
	initialized := make(chan struct{})
	go func() {
		defer func() {
			if client := sseClient.Load(); client != nil {
				client.Close()
			}
			logger.Debug("SSE client routine stopped.")
		}()

		for {
			logger.Debug("Starting SSE client routine")
			client := NewSSEClient(url, initCallback, errorCallback)

			if oldClient := sseClient.Swap(client); oldClient != nil {
				oldClient.Close()
			}

			if !client.Init() {
				logger.Debug("SSE client failed to initialize, restarting...")
				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}

			select {
			case <-initialized:
			default:
				close(initialized)
			}

			// Wait for disconnect or cancellation
			select {
			case <-client.ReconnectChan:
				logger.Debug("mcp SSE server cannot connect, restarting...")
			case <-ctx.Done():
				return
			}

			// Wait before trying to reconnect
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case <-initialized:
	case <-ctx.Done():
	}
}

func (c *MCPSSEClient) GetTools() ([]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("获取工具超时: %w", ctx.Err())
		case <-ticker.C:
			if len(c.tools) > 0 {
				return c.tools, nil
			}
		}
	}
}

type FunctionCall = types.FunctionCall
type LLMCallableTool = types.LLMCallableTool
type InputSchema = types.InputSchema
type ModelContextFunctionTool = types.ModelContextFunctionTool

type SSEFuncCall struct {
	MCPSSEClient     *atomic.Pointer[MCPSSEClient]
	OriginalMcpTools []ModelContextFunctionTool
	LLMCallableTools []LLMCallableTool
}

func ConvertViaJSON(src, dst any) error {
	// 序列化为JSON
	logger.Debugf("start time %v", time.Now())
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	// 从JSON反序列化为目标类型
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("反序列化失败: %w", err)
	}
	logger.Debugf("end time %v", time.Now())
	return nil

}

func (h *SSEFuncCall) GetTools() []LLMCallableTool {
	return h.LLMCallableTools
}

func (h *SSEFuncCall) SSEInitHandler(_ *MCPSSEClient) {

}

func (h *SSEFuncCall) UpdateTools() error {
	toolsAny, err := h.MCPSSEClient.Load().GetTools()
	if err != nil {
		fmt.Println("获取工具失败:", err)
		return err
	}

	logger.Debugf("toolsAny :%+v", toolsAny)
	if err := ConvertViaJSON(toolsAny, &h.OriginalMcpTools); err != nil {
		logger.Debugf("转换失败:%+v", err)
		return err
	}

	for _, v := range h.OriginalMcpTools {
		h.LLMCallableTools = append(h.LLMCallableTools, LLMCallableTool{
			Type:     "function",
			Function: v,
		})
	}

	return nil
}

func (h *SSEFuncCall) CallTool(call *FunctionCall) (string, error) {

	found := false
	for _, t := range h.OriginalMcpTools {
		if t.Name == call.Name {
			found = true
			break
		}
	}

	if !found {
		return "工具名未找到", fmt.Errorf("tool with name '%s' not found", call.Name)
	}

	success, result, err := h.MCPSSEClient.Load().CallFunction(call.Name, call.Arguments)
	if err != nil {
		return "调用工具发生错误", err
	}
	if !success {
		return fmt.Sprintf("调用工具 %s 失败 ", call.Name), fmt.Errorf("call function %s failed", call.Name)
	}
	return result, nil

}

func NewSSEFuncCall(ctx context.Context, sseURL string) (*SSEFuncCall, error) {
	var sseClient atomic.Pointer[MCPSSEClient]
	usercall := &SSEFuncCall{
		MCPSSEClient:     &sseClient,
		OriginalMcpTools: make([]ModelContextFunctionTool, 0),
		LLMCallableTools: make([]LLMCallableTool, 0),
	}

	RunRoutine(ctx, sseURL, &sseClient, usercall.SSEInitHandler)
	err := usercall.UpdateTools()
	return usercall, err
}
