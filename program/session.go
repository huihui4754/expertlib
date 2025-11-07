package programs

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/types"
)

const (
	SocketDir       = "/tmp/program_sockets"
	IdleTimeout     = 2 * time.Hour
	ProtocolMagic   = 0xDEADBEEF
	ProtocolVersion = 1
	HeaderSize      = 16
)

type Session struct {
	DialogID          string
	UserID            string
	Cmd               *exec.Cmd
	NodeJSProgramPath string
	SocketPath        string
	LastAccess        time.Time
	timer             *time.Timer
	dataPort          string
	mu                sync.Mutex
	manager           *SessionManager
	listener          net.Listener
	conn              net.Conn
	connMu            sync.Mutex
}

type SessionManager struct {
	sessions               map[string]*Session
	mu                     sync.RWMutex
	toExpertMessageOutChan chan *types.TotalMessage
	ProgramBasePath        string
}

func NewSessionManager(toExpertMessageOutChan chan *types.TotalMessage) *SessionManager {
	return &SessionManager{
		sessions:               make(map[string]*Session),
		toExpertMessageOutChan: toExpertMessageOutChan,
	}
}

func (m *SessionManager) GetOrCreateSession(dialogID string, userID string, intent string, httpPort string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[dialogID]; exists {
		logger.Infof("Found existing session for dialog_id: %s", dialogID)
		session.resetTimeout()
		return session, nil
	}

	logger.Infof("Creating new session for dialog_id: %s", dialogID)
	socketPath := filepath.Join(SocketDir, fmt.Sprintf("%s.sock", dialogID))
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		logger.Warnf("Could not remove old socket file %s: %v", socketPath, err)
	}

	NodeJSProgramPath := filepath.Join(m.ProgramBasePath, intent, fmt.Sprintf("%s.js", intent))

	if _, err := os.Stat(NodeJSProgramPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("program for intent '%s' not found at %s", intent, NodeJSProgramPath)
	}

	session := &Session{
		DialogID:          dialogID,
		UserID:            userID,
		NodeJSProgramPath: NodeJSProgramPath,
		SocketPath:        socketPath,
		LastAccess:        time.Now(),
		dataPort:          httpPort,
		manager:           m,
	}

	m.sessions[dialogID] = session

	err := session.start()
	if err != nil {
		delete(m.sessions, dialogID)
		return nil, err
	}

	return session, nil
}

func (m *SessionManager) CloseSession(dialogID string, reason int) {

	m.mu.Lock()
	session, exists := m.sessions[dialogID]
	m.mu.Unlock()
	if !exists {
		return
	}
	logger.Infof("Closing session for dialog_id: %s with reason: %d", dialogID, reason)
	session.close()

	m.mu.Lock()
	delete(m.sessions, dialogID)
	m.mu.Unlock()

	logger.Infof("Session %s closed.", dialogID)
}

func (m *SessionManager) GetAllProgramName() []string {
	entries, err := os.ReadDir(m.ProgramBasePath)
	if err != nil {
		logger.Errorf("read program dir err %v", err)
		return nil
	}

	var program []string
	for _, entry := range entries {
		// 判断是否为目录（且不是符号链接，若需包含符号链接目录可去掉 IsDir() 的参数）
		if entry.IsDir() {
			dirName := entry.Name()
			dirPath := filepath.Join(m.ProgramBasePath, dirName)
			_, err := os.ReadDir(dirPath)
			if err != nil {
				logger.Warnf("警告：无法读取目录 %q，原因：%v（已跳过）\n", dirPath, err)
				continue
			}

			targetFilePath := filepath.Join(m.ProgramBasePath, dirName, dirName+".js")

			// 检查该文件是否存在且是文件（不是目录）
			fileInfo, err := os.Stat(targetFilePath)
			if err != nil {
				// 如果是"文件不存在"的错误，跳过；其他错误（如权限问题）打印警告
				if !os.IsNotExist(err) {
					logger.Warnf("警告：检查文件 %q 时出错：%v（已跳过）\n", targetFilePath, err)
				}
				continue
			}

			// 确认是文件（不是目录）
			if !fileInfo.IsDir() {
				program = append(program, dirName)
			}
		}
	}

	return program
}

func (s *Session) listenOnSocket() {
	// time.Sleep(100 * time.Millisecond)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			logger.Warnf("Error accepting connection for dialog %s: %v", s.DialogID, err)
			return // Stop listening if accept fails (e.g., listener closed)
		}

		s.connMu.Lock()
		if s.conn != nil {
			s.conn.Close()
		}
		s.conn = conn
		s.connMu.Unlock()

		go s.handleConnection(conn)
	}
}

func (s *Session) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		s.connMu.Lock()
		if s.conn == conn {
			s.conn = nil
		}
		s.connMu.Unlock()
	}()

	for {
		s.resetTimeout()

		header := make([]byte, HeaderSize)
		if _, err := io.ReadFull(conn, header); err != nil {
			if err != io.EOF {
				logger.Errorf("Error reading header for dialog %s: %v", s.DialogID, err)
			}
			return
		}

		if magic := binary.BigEndian.Uint32(header[0:4]); magic != ProtocolMagic {
			logger.Errorf("Invalid magic number for dialog %s: got %x", s.DialogID, magic)
			return
		}

		bodyLength := binary.BigEndian.Uint32(header[8:12])
		body := make([]byte, bodyLength)

		if _, err := io.ReadFull(conn, body); err != nil {
			logger.Errorf("Error reading body for dialog %s: %v", s.DialogID, err)
			return
		}

		var totalMsg types.TotalMessage
		if err := json.Unmarshal(body, &totalMsg); err != nil {
			logger.Errorf("Failed to unmarshal message from tool for dialog %s: %v", s.DialogID, err)
			continue
		}

		logger.Debugf("Received message from tool for dialog %s, event: %d", s.DialogID, totalMsg.EventType)

		switch totalMsg.EventType {
		case types.EventServerMessage:
			s.manager.toExpertMessageOutChan <- &totalMsg
		case types.EventToolFinish, types.EventToolNotSupport:
			s.manager.toExpertMessageOutChan <- &totalMsg
			s.manager.CloseSession(s.DialogID, totalMsg.EventType)
			return
		default:
			logger.Errorf("返回不支持的消息 ： %v", totalMsg)
			s.manager.toExpertMessageOutChan <- &totalMsg
		}
	}
}

func (s *Session) waitForProcess() {
	err := s.Cmd.Wait()
	if err != nil {
		logger.Warnf("Node.js process for dialog %s exited with error: %v", s.DialogID, err)
	} else {
		logger.Infof("Node.js process for dialog %s exited gracefully.", s.DialogID)
	}
	s.manager.CloseSession(s.DialogID, types.EventToolFinish)
}

func (s *Session) close() {
	if s.timer != nil {
		s.timer.Stop()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		if err := s.Cmd.Process.Kill(); err != nil {
			logger.Warnf("Failed to kill process for dialog_id %s: %v", s.DialogID, err)
		}
	}
	if err := os.Remove(s.SocketPath); err != nil && !os.IsNotExist(err) {
		logger.Warnf("Failed to remove socket file %s: %v", s.SocketPath, err)
	}
}

func (s *Session) start() error {
	s.Cmd = exec.Command("node", s.NodeJSProgramPath, fmt.Sprintf("--socket=%s", s.SocketPath), fmt.Sprintf("--port=%s", s.dataPort))
	s.Cmd.Stdout = os.Stdout
	s.Cmd.Stderr = os.Stderr

	listener, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		logger.Errorf("Failed to listen on socket for dialog %s: %v", s.DialogID, err)
		return err
	}
	s.listener = listener

	if err := s.Cmd.Start(); err != nil {
		s.listener.Close() // Clean up listener if process fails to start
		logger.Errorf("failed to start nodejs program: %w", err)
		return err
	}

	logger.Infof("Listening on socket %s for dialog %s", s.SocketPath, s.DialogID)

	go s.listenOnSocket()
	go s.waitForProcess()
	return nil
}

func (s *Session) Send(message *types.TotalMessage) error {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	// Wait for the Node.js process to connect, holding the lock.
	// This is not ideal for performance but is simple and safe from races.
	if s.conn == nil {
		for i := 0; i < 10; i++ { // Retry for up to 1 second
			s.connMu.Unlock()
			time.Sleep(100 * time.Millisecond)
			s.connMu.Lock()
			if s.conn != nil {
				break
			}
		}
	}

	if s.conn == nil {
		return fmt.Errorf("failed to send message: no active connection to nodejs process")
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	header := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(header[0:4], ProtocolMagic)
	binary.BigEndian.PutUint16(header[4:6], ProtocolVersion)
	binary.BigEndian.PutUint16(header[6:8], uint16(message.EventType))
	binary.BigEndian.PutUint32(header[8:12], uint32(len(body)))

	if _, err := s.conn.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := s.conn.Write(body); err != nil {
		return fmt.Errorf("failed to write body: %w", err)
	}

	return nil
}

func (s *Session) resetTimeout() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(IdleTimeout, func() {
		logger.Infof("Session for dialog_id %s timed out due to inactivity.", s.DialogID)
		s.manager.CloseSession(s.DialogID, types.EventToolFinish)
	})
	s.LastAccess = time.Now()
}

func init() {
	if err := os.MkdirAll(SocketDir, 0755); err != nil {
		logger.Fatalf("Failed to create socket directory: %v", err)
		panic("无法在tmp 中创建sock 目录")
	}
}
