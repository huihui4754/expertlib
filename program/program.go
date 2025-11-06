package programs

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/huihui4754/expertlib/types"
	"github.com/huihui4754/loglevel"
)

const (
	MagicNumber       = 0x1A2B3C4D
	ProtocolVersion   = 1
	HeartbeatInterval = 10 * time.Second
	MessageTimeout    = 5 * time.Second
	DefaultPort       = "8083"
	SaveInterval      = 5 * time.Minute
)

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

type TotalMessage = types.TotalMessage

func SetLogger(level loglevel.Level) {
	logger.SetLevel(level)
}

type program struct {
	dataFilePath           string
	programPath            string
	expertMessageHandler   func(any, string)
	expertMessageInChan    chan *TotalMessage
	toExpertMessageOutChan chan *TotalMessage
	sessions               *sessionManager
	httpPort               string
	dataStore              map[string]map[string]string // user_id -> key -> value
	dataStoreMu            sync.Mutex
}

type sessionManager struct {
	sessions map[string]*session
	mu       sync.Mutex
}

func (sm *sessionManager) Add(s *session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[s.dialogID] = s
}

func (sm *sessionManager) Get(dialogID string) (*session, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.sessions[dialogID]
	return s, ok
}

func (sm *sessionManager) Remove(dialogID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, dialogID)
}

type session struct {
	dialogID      string
	userID        string
	programName   string
	cmd           *exec.Cmd
	socketPath    string
	conn          net.Conn
	cancel        context.CancelFunc
	messageInChan chan *TotalMessage
	lastActive    time.Time
	mu            sync.Mutex
	program       *program
}

type MessageHeader struct {
	Magic      uint32
	Version    uint16
	Type       uint16
	BodyLength uint32
	Reserved   uint32
}

func NewTool() *program {
	defalutProgramPath := ""
	defalutDataPath := ""
	currentUser, err := user.Current() // todo 后续支持从配置文件读取配置
	if err != nil {
		fmt.Printf("获取用户信息失败：%v\n", err)
	} else {
		defalutProgramPath = filepath.Join(currentUser.HomeDir, "expert", "program")
		defalutDataPath = filepath.Join(currentUser.HomeDir, "expert", "js")
	}

	p := &program{
		dataFilePath:           defalutDataPath,
		programPath:            defalutProgramPath,
		expertMessageInChan:    make(chan *TotalMessage),
		toExpertMessageOutChan: make(chan *TotalMessage),
		sessions: &sessionManager{
			sessions: make(map[string]*session),
		},
		httpPort:  DefaultPort,
		dataStore: make(map[string]map[string]string),
	}

	p.loadData()
	go p.startHttpServer()

	return p
}

func (p *program) loadData() {
	p.dataStoreMu.Lock()
	defer p.dataStoreMu.Unlock()

	dataPath := filepath.Join(p.dataFilePath, "datastore.json")
	file, err := os.Open(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Data file not found, starting with an empty data store.")
			return
		}
		logger.Errorf("Failed to open data file: %v", err)
		return
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&p.dataStore); err != nil {
		logger.Errorf("Failed to decode data file: %v", err)
	}
}

func (p *program) saveData() {
	p.dataStoreMu.Lock()
	defer p.dataStoreMu.Unlock()

	dataPath := filepath.Join(p.dataFilePath, "datastore.json")
	if err := os.MkdirAll(p.dataFilePath, 0755); err != nil {
		logger.Errorf("Failed to create data directory: %v", err)
		return
	}

	file, err := os.Create(dataPath)
	if err != nil {
		logger.Errorf("Failed to create data file: %v", err)
		return
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(p.dataStore); err != nil {
		logger.Errorf("Failed to encode data file: %v", err)
	}
}

func (p *program) SetDataFilePath(path string) {
	p.dataFilePath = path
	logger.Info("Data file path set to:", path)
}

func (p *program) SetProgramPath(path string) {
	p.programPath = path
	logger.Info("Program path set to:", path)
}

func (p *program) HandleExpertRequestMessage(message any) {
	logger.Debugf("Handling Expert request message: %v", message)
	var messagePointer *TotalMessage
	var err error
	switch v := message.(type) {
	case TotalMessage:
		// 复制值类型，取新地址
		msg := v
		messagePointer = &msg
	case *TotalMessage:
		if v == nil {
			logger.Error("*TotalMessage 为 nil")
			break
		}
		// 复制指针指向的值，取新地址（避免外部修改影响）
		msg := *v // 解引用并复制
		messagePointer = &msg
	case string:
		var totalMsg TotalMessage
		err = json.Unmarshal([]byte(v), &totalMsg)
		if err == nil {
			messagePointer = &totalMsg
		} else {
			logger.Errorf("无法解析字符串消息为 TotalMessage  message: %v,  err: %v", v, err)
		}
	case []byte:
		var totalMsg TotalMessage
		err = json.Unmarshal(v, &totalMsg)
		if err == nil {
			messagePointer = &totalMsg
		} else {
			logger.Errorf("无法解析bytes消息为 TotalMessage  bytes: %v,  err: %v", string(v), err)
		}

	default:
		logger.Error("不支持的消息结构")
	}
	if p.expertMessageInChan != nil && messagePointer != nil && err == nil {
		p.expertMessageInChan <- messagePointer
	}
}

func (p *program) SetToExpertMessageHandler(handler func(any, string)) {
	p.expertMessageHandler = handler
	logger.Info("ExpertMessageHandler set")
}

func (p *program) handleFromExpertMessage(message *TotalMessage) {

	switch message.EventType {
	case 1001:
		session, ok := p.sessions.Get(message.DialogID)
		if !ok {
			// todo 确定意图
			intent := "hello"
			var err error
			session, err = newSession(p, message.DialogID, message.UserId, intent)
			if err != nil {
				logger.Errorf("Failed to create session: %v", err)
				return
			}
			p.sessions.Add(session)
			go session.run()
		}
		session.messageInChan <- message

	case 1002:
		logger.Debug("专家终止对话")
		if session, ok := p.sessions.Get(message.DialogID); ok {
			session.stop()
		}

	default:
		log.Printf("收到未知事件类型: %d", message.EventType)
	}

}

func newSession(p *program, dialogID, userID, programName string) (*session, error) {
	ctx, cancel := context.WithCancel(context.Background())
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("expert-%s.sock", uuid.New().String()))

	s := &session{
		dialogID:      dialogID,
		userID:        userID,
		programName:   programName,
		socketPath:    socketPath,
		cancel:        cancel,
		messageInChan: make(chan *TotalMessage, 10),
		lastActive:    time.Now(),
		program:       p,
	}

	programPath := filepath.Join(p.programPath, programName, programName+".js")
	cmd := exec.CommandContext(ctx, "node", programPath, "--socket="+socketPath, "--port="+p.httpPort)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	s.cmd = cmd

	return s, nil
}

func (s *session) run() {
	defer s.stop()

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		logger.Errorf("Failed to listen on socket: %v", err)
		return
	}
	defer listener.Close()

	if err := s.cmd.Start(); err != nil {
		logger.Errorf("Failed to start program: %v", err)
		return
	}

	connChan := make(chan net.Conn)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			logger.Errorf("Failed to accept connection: %v", err)
			return
		}
		connChan <- conn
	}()

	select {
	case s.conn = <-connChan:
		logger.Infof("Program connected for dialog %s", s.dialogID)
	case <-time.After(10 * time.Second):
		logger.Errorf("Program connection timeout for dialog %s", s.dialogID)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.handleProgramMessages(ctx)

	idleTimer := time.NewTimer(3 * time.Hour) // todo 从配置读取
	defer idleTimer.Stop()

	for {
		select {
		case msg := <-s.messageInChan:
			s.lastActive = time.Now()
			idleTimer.Reset(3 * time.Hour)
			if err := s.sendMessageToProgram(msg); err != nil {
				logger.Errorf("Failed to send message to program: %v", err)
				return
			}
		case <-idleTimer.C:
			logger.Infof("Session %s timed out", s.dialogID)
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *session) stop() {
	s.program.sessions.Remove(s.dialogID)
	if s.cancel != nil {
		s.cancel()
	}
	if s.conn != nil {
		s.conn.Close()
	}
	os.Remove(s.socketPath)
	logger.Infof("Session %s stopped", s.dialogID)
}

func (s *session) sendMessageToProgram(msg *TotalMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := MessageHeader{
		Magic:      MagicNumber,
		Version:    ProtocolVersion,
		Type:       uint16(msg.EventType),
		BodyLength: uint32(len(body)),
	}

	if err := binary.Write(s.conn, binary.BigEndian, &header); err != nil {
		return err
	}

	if _, err := s.conn.Write(body); err != nil {
		return err
	}

	return nil
}

func (s *session) handleProgramMessages(ctx context.Context) {
	reader := bufio.NewReader(s.conn)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var header MessageHeader
			if err := binary.Read(reader, binary.BigEndian, &header); err != nil {
				if err == io.EOF {
					logger.Infof("Program disconnected for dialog %s", s.dialogID)
					s.stop()
					return
				}
				logger.Errorf("Failed to read message header: %v", err)
				return
			}

			if header.Magic != MagicNumber {
				logger.Errorf("Invalid magic number: %x", header.Magic)
				return
			}

			body := make([]byte, header.BodyLength)
			if _, err := io.ReadFull(reader, body); err != nil {
				logger.Errorf("Failed to read message body: %v", err)
				return
			}

			var msg TotalMessage
			if err := json.Unmarshal(body, &msg); err != nil {
				logger.Errorf("Failed to unmarshal program message: %v", err)
				continue
			}

			s.handleMessageFromProgram(&msg)
		}
	}
}

func (s *session) handleMessageFromProgram(msg *TotalMessage) {
	switch msg.EventType {
	case 2001, 2003:
		s.program.toExpertMessageOutChan <- msg
	case 2002:
		s.program.toExpertMessageOutChan <- msg
		s.stop()
	default:
		logger.Warnf("Unknown event type from program: %d", msg.EventType)
	}
}

func (p *program) startHttpServer() {
	http.HandleFunc("/", p.handleHttpRequest)
	logger.Infof("Starting HTTP server on port %s", p.httpPort)
	if err := http.ListenAndServe(":"+p.httpPort, nil); err != nil {
		logger.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func (p *program) handleHttpRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EventType int    `json:"event_type"`
		Action    string `json:"action"`
		Key       string `json:"key"`
		Value     string `json:"value,omitempty"`
		UserID    string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid request body")
		return
	}

	if req.EventType != 3000 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid event type")
		return
	}

	p.dataStoreMu.Lock()
	defer p.dataStoreMu.Unlock()

	if _, ok := p.dataStore[req.UserID]; !ok {
		p.dataStore[req.UserID] = make(map[string]string)
	}

	switch req.Action {
	case "query_tool_memory":
		value := p.dataStore[req.UserID][req.Key]
		resp := struct {
			EventType int    `json:"event_type"`
			Action    string `json:"action"`
			Key       string `json:"key"`
			Value     string `json:"value"`
		}{
			EventType: 3000,
			Action:    "get_tool_memory",
			Key:       req.Key,
			Value:     value,
		}
		json.NewEncoder(w).Encode(resp)

	case "save_tool_memory":
		p.dataStore[req.UserID][req.Key] = req.Value
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid action")
	}
}

func (p *program) Run() {
	ticker := time.NewTicker(SaveInterval)
	defer ticker.Stop()

	logger.Info("Program instance running")
	for {
		select {
		case expertMsg := <-p.expertMessageInChan:
			go p.handleFromExpertMessage(expertMsg)
		case toExpertMsg := <-p.toExpertMessageOutChan:
			go func() {
				toExpertMessage := *toExpertMsg
				msg, err := json.Marshal(toExpertMessage)
				if err != nil {
					logger.Error("Failed to marshal chat message: %v", err)
				}
				p.expertMessageHandler(toExpertMessage, string(msg))
			}()
		case <-ticker.C:
			p.saveData()
		}
	}

}

func (p *program) GetProgramNames() []string {
	logger.Debug("Getting all program names")
	// Placeholder for actual logic
	return []string{"program1", "program2"}
}

func (p *program) UpdatePrograms() {
	logger.Info("Updating program")
	// Placeholder for actual logic
}
