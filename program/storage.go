package programs

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/huihui4754/expertlib/types"
)

const (
	Port = "8083"
)

type HttpInstruction = types.HttpInstruction

type DialogData struct {
	data     map[string]any
	cacheMd5 string
	mu       sync.RWMutex
}

type StorageManager struct {
	data        map[string]*DialogData
	mu          sync.RWMutex
	DataDirPath string
	Port        string
}

func NewStorage(dataDirPath string, port string) *StorageManager {
	storage := &StorageManager{
		data:        make(map[string]*DialogData),
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
	go s.periodicPersist()
	http.HandleFunc("/memory", s.memoryHandler)
	logger.Printf("Starting HTTP server on port %s", Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", Port), nil); err != nil {
		logger.Fatalf("HTTP server failed: %v", err)
	}
}

func (s *StorageManager) GetStroageHandler() func(w http.ResponseWriter, r *http.Request) {
	return s.memoryHandler
}

func (s *StorageManager) periodicPersist() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for dialogID := range s.data {
			s.persistDialogData(dialogID)

		}
		s.mu.Unlock()
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

	if _, ok := s.data[req.DialogID]; !ok {
		err := s.loadDialogData(req.DialogID)
		if err != nil {
			s.data[req.DialogID] = &DialogData{
				data:     map[string]any{},
				cacheMd5: "",
				mu:       sync.RWMutex{},
			}
		}
	}
	s.data[req.DialogID].data[req.Key] = req.Value

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

	// Load data from disk if not in memory
	if _, ok := s.data[dialogID]; !ok {
		if err := s.loadDialogData(dialogID); err != nil {
			// It's not an error if the file doesn't exist yet
			if !os.IsNotExist(err) {
				logger.Printf("Error loading data for dialog %s: %v", dialogID, err)
			}
		}
	}

	var value any
	if data, ok := s.data[dialogID]; ok {
		if val, ok := data.data[key]; ok {
			value = val
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
	s.data[dialogID].mu.Lock()
	data, err := json.MarshalIndent(s.data[dialogID].data, "", "  ")
	s.data[dialogID].mu.Unlock()
	if err != nil {
		return err
	}
	hash := md5.Sum(data)
	currentMd5 := hex.EncodeToString(hash[:])
	if currentMd5 == s.data[dialogID].cacheMd5 {
		logger.Debug("无需保存")
		return nil
	}
	return os.WriteFile(filePath, data, 0644)
}

func (s *StorageManager) loadDialogData(dialogID string) error {
	filePath := filepath.Join(s.DataDirPath, fmt.Sprintf("%s.json", dialogID))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var dialogData map[string]any
	if err := json.Unmarshal(data, &dialogData); err != nil {
		return err
	}
	hash := md5.Sum(data)
	currentMd5 := hex.EncodeToString(hash[:])
	dialog := &DialogData{
		data:     dialogData,
		cacheMd5: currentMd5,
		mu:       sync.RWMutex{},
	}

	s.mu.Lock()
	s.data[dialogID] = dialog
	s.mu.Unlock()
	return nil
}
