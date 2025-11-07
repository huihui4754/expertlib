package programs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/huihui4754/expertlib/types"
)

const (
	Port = "8083"
)

type HttpInstruction = types.HttpInstruction

type StorageManager struct {
	data        map[string]map[string]interface{}
	mu          sync.RWMutex
	DataDirPath string
	Port        string
}

func NewStorage(dataDirPath string, port string) *StorageManager {
	storage := &StorageManager{
		data:        make(map[string]map[string]interface{}),
		DataDirPath: dataDirPath,
		mu:          sync.RWMutex{},
		Port:        port,
	}

	return storage
}

func (s *StorageManager) RunHTTPServer() {
	if err := os.MkdirAll(s.DataDirPath, 0755); err != nil {
		logger.Fatalf("Failed to create data directory: %v", err)
		panic("无法创建存储目录")
	}
	http.HandleFunc("/memory", s.memoryHandler)
	logger.Printf("Starting HTTP server on port %s", Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", Port), nil); err != nil {
		logger.Fatalf("HTTP server failed: %v", err)
	}
}

func (s *StorageManager) memoryHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.saveMemory(w, r)
	case http.MethodGet:
		s.queryMemory(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *StorageManager) saveMemory(w http.ResponseWriter, r *http.Request) {
	var req HttpInstruction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DialogID == "" || req.Key == "" {
		http.Error(w, "dialog_id and key are required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[req.DialogID]; !ok {
		s.data[req.DialogID] = make(map[string]interface{})
	}
	s.data[req.DialogID][req.Key] = req.Value

	if err := s.persistDialogData(req.DialogID); err != nil {
		http.Error(w, "Failed to save data", http.StatusInternalServerError)
		logger.Printf("Error persisting data for dialog %s: %v", req.DialogID, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *StorageManager) queryMemory(w http.ResponseWriter, r *http.Request) {
	dialogID := r.URL.Query().Get("dialog_id")
	key := r.URL.Query().Get("key")

	if dialogID == "" || key == "" {
		http.Error(w, "dialog_id and key are required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Load data from disk if not in memory
	if _, ok := s.data[dialogID]; !ok {
		if err := s.loadDialogData(dialogID); err != nil {
			// It's not an error if the file doesn't exist yet
			if !os.IsNotExist(err) {
				logger.Printf("Error loading data for dialog %s: %v", dialogID, err)
			}
		}
	}

	value := ""
	if data, ok := s.data[dialogID]; ok {
		if val, ok := data[key]; ok {
			value = fmt.Sprintf("%v", val)
		}
	}

	resp := HttpInstruction{
		EventType: types.EventSpecialInstruction,
		Action:    "get_tool_memory",
		Key:       key,
		Value:     value,
		DialogID:  dialogID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *StorageManager) persistDialogData(dialogID string) error {
	filePath := filepath.Join(s.DataDirPath, fmt.Sprintf("%s.json", dialogID))
	data, err := json.MarshalIndent(s.data[dialogID], "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func (s *StorageManager) loadDialogData(dialogID string) error {
	filePath := filepath.Join(s.DataDirPath, fmt.Sprintf("%s.json", dialogID))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var dialogData map[string]interface{}
	if err := json.Unmarshal(data, &dialogData); err != nil {
		return err
	}

	s.data[dialogID] = dialogData
	return nil
}
