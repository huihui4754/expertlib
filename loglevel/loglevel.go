package loglevel

import (
	"log"
	"os"
	"runtime"
	"strings"
)

type MyLog struct {
	*log.Logger
	level    int
	fileName string
}

const (
	Debug int = iota
	Info
	Warn
	Error
)

const (
	DebugStr = "[DEBUG]"
	InfoStr  = "[INFO]"
	WarnStr  = "[WARN]"
	ErrorStr = "[ERROR]"
)

var levelStr = map[int]string{
	Debug: DebugStr,
	Info:  InfoStr,
	Warn:  WarnStr,
	Error: ErrorStr,
}

func getCallerFileName() string {
	_, file, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	// 提取文件名（不含路径）
	parts := strings.Split(file, "/")
	fileName := parts[len(parts)-1]
	return fileName
}

func NewLog(level int) *MyLog {

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime) // 保留日期和时间
	myLog := MyLog{
		Logger:   logger,
		level:    level,
		fileName: getCallerFileName(),
	}
	return &myLog
}

func (l *MyLog) SetLevel(level int) {
	l.level = level
}

func (l *MyLog) Info(msgArr ...any) {
	l.print(Info, msgArr...)
}

func (l *MyLog) Debug(msgArr ...any) {
	l.print(Debug, msgArr...)
}

func (l *MyLog) Warn(msgArr ...any) {
	l.print(Warn, msgArr...)
}

func (l *MyLog) Error(msgArr ...any) {
	l.print(Error, msgArr...)
}

func (l *MyLog) Infof(format string, msgArr ...any) {
	l.printf(Info, format, msgArr...)
}

func (l *MyLog) Debugf(format string, msgArr ...any) {
	l.printf(Debug, format, msgArr...)
}

func (l *MyLog) Warnf(format string, msgArr ...any) {
	l.printf(Warn, format, msgArr...)
}

func (l *MyLog) Errorf(format string, msgArr ...any) {
	l.printf(Error, format, msgArr...)
}

func (l *MyLog) print(level int, msgArr ...any) {
	if l.level <= level {
		levelStr := levelStr[level]
		// 构造包含文件名的消息前缀
		prefix := []any{levelStr, "[", l.fileName, "]: "}
		l.Logger.Print(append(prefix, msgArr...)...)
	}
}

func (l *MyLog) printf(level int, format string, msgArr ...any) {
	if l.level <= level {
		levelStr := levelStr[level]
		// 构造包含文件名的格式化前缀
		format = levelStr + " [" + l.fileName + "]: " + format
		l.Logger.Printf(format, msgArr...)
	}
}
