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

type HttpInstruction = types.HttpInstruction

type DialogData struct {
	data     map[string]any
	cacheMd5 string
	mu       sync.RWMutex
}

type StorageManager struct {
	data         map[string]*DialogData
	mu           sync.RWMutex
	DataDirPath  string
	Port         string
	SaveInterval time.Duration
}

func NewStorage(dataDirPath string, port string) *StorageManager {
	storage := &StorageManager{
		data:         make(map[string]*DialogData),
		DataDirPath:  dataDirPath,
		mu:           sync.RWMutex{},
		Port:         port,
		SaveInterval: 10 * time.Minute,
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
	logger.Printf("Starting HTTP server on port %s", s.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", s.Port), nil); err != nil {
		logger.Fatalf("HTTP server failed: %v", err)
	}
}

func (s *StorageManager) GetStroageHandler() func(w http.ResponseWriter, r *http.Request) {
	return s.memoryHandler
}

func (s *StorageManager) periodicPersist() {
	ticker := time.NewTicker(s.SaveInterval)
	defer ticker.Stop()

	for range ticker.C {
		for dialogID := range s.data {
			s.persistDialogData(dialogID)

		}
	}
}

func (s *StorageManager) memoryHandler(w http.ResponseWriter, r *http.Request) {
	var req HttpInstruction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DialogID == "" || req.Key == "" {
		http.Error(w, "dialog_id and key are required", http.StatusBadRequest)
		return
	}

	dialog := s.getOrCreateDialogData(req.DialogID)

	switch req.Action {
	case "query_tool_memory":
		dialog.mu.RLock()
		value := dialog.data[req.Key]
		dialog.mu.RUnlock()

		resp := HttpInstruction{
			EventType: types.EventSpecialInstruction,
			Action:    "get_tool_memory",
			Key:       req.Key,
			Value:     value,
			DialogID:  req.DialogID,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case "save_tool_memory":
		dialog.mu.Lock()
		dialog.data[req.Key] = req.Value
		dialog.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}
}

func (s *StorageManager) getOrCreateDialogData(dialogID string) *DialogData {
	s.mu.RLock()
	dialog, ok := s.data[dialogID]
	s.mu.RUnlock()
	if ok {
		return dialog
	}

	dialog, err := s.loadDialogDataFromFile(dialogID)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Printf("Error loading data for dialog %s: %v", dialogID, err)
		}
		// If file doesn't exist or fails to load, create a new one
		dialog = &DialogData{
			data:     make(map[string]any),
			cacheMd5: "",
			mu:       sync.RWMutex{},
		}
	}

	s.mu.RLock()
	s.data[dialogID] = dialog
	s.mu.RUnlock()
	return dialog
}

func (s *StorageManager) persistDialogData(dialogID string) error {
	dialog := s.data[dialogID] // This is safe because periodicPersist holds the lock on s.mu

	dialog.mu.Lock()
	defer dialog.mu.Unlock()

	data, err := json.MarshalIndent(dialog.data, "", "  ")
	if err != nil {
		return err
	}

	hash := md5.Sum(data)
	currentMd5 := hex.EncodeToString(hash[:])
	if currentMd5 == dialog.cacheMd5 {
		logger.Debug("无需保存")
		return nil
	}

	filePath := filepath.Join(s.DataDirPath, fmt.Sprintf("%s.json", dialogID))
	err = os.WriteFile(filePath, data, 0644)
	if err == nil {
		dialog.cacheMd5 = currentMd5
	}
	return err
}

func (s *StorageManager) loadDialogDataFromFile(dialogID string) (*DialogData, error) {
	filePath := filepath.Join(s.DataDirPath, fmt.Sprintf("%s.json", dialogID))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var dialogData map[string]any
	if err := json.Unmarshal(data, &dialogData); err != nil {
		return nil, err
	}
	hash := md5.Sum(data)
	currentMd5 := hex.EncodeToString(hash[:])
	dialog := &DialogData{
		data:     dialogData,
		cacheMd5: currentMd5,
		mu:       sync.RWMutex{},
	}

	return dialog, nil
}
