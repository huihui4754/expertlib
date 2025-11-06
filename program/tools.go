package tools

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/huihui4754/loglevel"
)

var (
	logger = loglevel.NewLog(loglevel.Debug)
)

func SetLogger(level loglevel.Level) {
	logger.SetLevel(level)
}

type Tool struct {
	dataFilePath         string
	programPath          string
	ExpertMessageHandler func(any, string)
}

func NewTool() *Tool {
	defalutProgramPath := ""
	defalutDataPath := ""
	currentUser, err := user.Current() // todo 后续支持从配置文件读取配置
	if err != nil {
		fmt.Printf("获取用户信息失败：%v\n", err)
	} else {
		defalutProgramPath = filepath.Join(currentUser.HomeDir, "expert", "program")
		defalutDataPath = filepath.Join(currentUser.HomeDir, "expert", "js")
	}
	return &Tool{
		dataFilePath: defalutDataPath,
		programPath:  defalutProgramPath,
	}
}

func (t *Tool) SetDataFilePath(path string) {
	t.dataFilePath = path
	logger.Info("Data file path set to:", path)
}

func (t *Tool) SetProgramPath(path string) {
	t.programPath = path
	logger.Info("Program path set to:", path)
}

func (t *Tool) HandleExpertRequestMessage(jsonx any) {
	logger.Debug("Handling Expert request message:", jsonx)
	if t.ExpertMessageHandler != nil {
		// Placeholder for actual logic
		t.ExpertMessageHandler("response from tool for Expert", "")
	}
}

func (t *Tool) HandleExpertRequestMessageString(message string) {
	logger.Debug("Handling Expert request message string:", message)
	if t.ExpertMessageHandler != nil {
		// Placeholder for actual logic
		t.ExpertMessageHandler("response from tool for Expert", "")
	}
}

func (t *Tool) SetToExpertMessageHandler(handler func(any, string)) {
	t.ExpertMessageHandler = handler
	logger.Info("ExpertMessageHandler set")
}

func (t *Tool) Run() {
	logger.Info("Tool instance running")
}

func (t *Tool) GetProgramNames() []string {
	logger.Debug("Getting all program names")
	// Placeholder for actual logic
	return []string{"program1", "program2"}
}

func (t *Tool) UpdatePrograms() {
	logger.Info("Updating program")
	// Placeholder for actual logic
}
