package programs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

const (
	Port = "8083"
)

type StorageManager struct {
	data       map[string]map[string]interface{}
	mu         sync.RWMutex
	dataDirPath string
}

var storage = &StorageManager{
	data: make(map[string]map[string]interface{}),
}

func InitStorage(dataDirPath string) {
	storage.dataDirPath = dataDirPath
	if err := os.MkdirAll(dataDirPath, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
}

func RunHTTPServer() {
	http.HandleFunc("/memory", memoryHandler)
	log.Printf("Starting HTTP server on port %s", Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", Port), nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func memoryHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		saveMemory(w, r)
	case http.MethodGet:
		queryMemory(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func saveMemory(w http.ResponseWriter, r *http.Request) {
	var req SpecialInstruction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DialogID == "" || req.Key == "" {
		http.Error(w, "dialog_id and key are required", http.StatusBadRequest)
		return
	}

	storage.mu.Lock()
	defer storage.mu.Unlock()

	if _, ok := storage.data[req.DialogID]; !ok {
		storage.data[req.DialogID] = make(map[string]interface{})
	}
	storage.data[req.DialogID][req.Key] = req.Value

	if err := persistDialogData(req.DialogID); err != nil {
		http.Error(w, "Failed to save data", http.StatusInternalServerError)
		log.Printf("Error persisting data for dialog %s: %v", req.DialogID, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func queryMemory(w http.ResponseWriter, r *http.Request) {
	dialogID := r.URL.Query().Get("dialog_id")
	key := r.URL.Query().Get("key")

	if dialogID == "" || key == "" {
		http.Error(w, "dialog_id and key are required", http.StatusBadRequest)
		return
	}

	storage.mu.RLock()
	defer storage.mu.RUnlock()

	// Load data from disk if not in memory
	if _, ok := storage.data[dialogID]; !ok {
		if err := loadDialogData(dialogID); err != nil {
			// It's not an error if the file doesn't exist yet
			if !os.IsNotExist(err) {
				log.Printf("Error loading data for dialog %s: %v", dialogID, err)
			}
		}
	}

	value := ""
	if data, ok := storage.data[dialogID]; ok {
		if val, ok := data[key]; ok {
			value = fmt.Sprintf("%v", val)
		}
	}

	resp := SpecialInstruction{
		EventType: EventSpecialInstruction,
		Action:    "get_tool_memory",
		Key:       key,
		Value:     value,
		DialogID:  dialogID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func persistDialogData(dialogID string) error {
	filePath := filepath.Join(storage.dataDirPath, fmt.Sprintf("%s.json", dialogID))
	data, err := json.MarshalIndent(storage.data[dialogID], "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, 0644)
}

func loadDialogData(dialogID string) error {
	filePath := filepath.Join(storage.dataDirPath, fmt.Sprintf("%s.json", dialogID))
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	var dialogData map[string]interface{}
	if err := json.Unmarshal(data, &dialogData); err != nil {
		return err
	}

	storage.data[dialogID] = dialogData
	return nil
}
