package loglevel

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestNewLog(t *testing.T) {
	log := NewLog(Info)
	if log == nil {
		t.Error("NewLog should not return nil")
	}
	if log.level != Info {
		t.Errorf("NewLog level should be %d, but got %d", Info, log.level)
	}
}

func TestSetLevel(t *testing.T) {
	log := NewLog(Info)
	log.SetLevel(Debug)
	if log.level != Debug {
		t.Errorf("SetLevel should change the level to %d, but got %d", Debug, log.level)
	}
}

func TestLogLevel(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	myLog := NewLog(Info)
	myLog.Logger.SetOutput(&buf)

	// Test Info level
	myLog.Info("test info")
	if !strings.Contains(buf.String(), "[INFO]") {
		t.Errorf("Info log should contain [INFO]")
	}
	buf.Reset()

	// Test Debug level (should not print)
	myLog.Debug("test debug")
	if buf.String() != "" {
		t.Errorf("Debug log should not be printed when level is Info")
	}
	buf.Reset()

	// Test Warn level
	myLog.Warn("test warn")
	if !strings.Contains(buf.String(), "[WARN]") {
		t.Errorf("Warn log should contain [WARN]")
	}
	buf.Reset()

	// Test Error level
	myLog.Error("test error")
	if !strings.Contains(buf.String(), "[ERROR]") {
		t.Errorf("Error log should contain [ERROR]")
	}
	buf.Reset()

	// Change level to Debug
	myLog.SetLevel(Debug)
	myLog.Debug("test debug")
	if !strings.Contains(buf.String(), "[DEBUG]") {
		t.Errorf("Debug log should be printed when level is Debug")
	}
	buf.Reset()
}

func TestLogLevelf(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	myLog := NewLog(Info)
	myLog.Logger.SetOutput(&buf)

	// Test Infof level
	myLog.Infof("test info %s", "f")
	if !strings.Contains(buf.String(), "[INFO]") {
		t.Errorf("Infof log should contain [INFO]")
	}
	if !strings.Contains(buf.String(), "test info f") {
		t.Errorf("Infof log should contain the formatted message")
	}
	buf.Reset()

	// Test Debugf level (should not print)
	myLog.Debugf("test debug %s", "f")
	if buf.String() != "" {
		t.Errorf("Debugf log should not be printed when level is Info")
	}
	buf.Reset()

	// Test Warnf level
	myLog.Warnf("test warn %s", "f")
	if !strings.Contains(buf.String(), "[WARN]") {
		t.Errorf("Warnf log should contain [WARN]")
	}
	if !strings.Contains(buf.String(), "test warn f") {
		t.Errorf("Warnf log should contain the formatted message")
	}
	buf.Reset()

	// Test Errorf level
	myLog.Errorf("test error %s", "f")
	if !strings.Contains(buf.String(), "[ERROR]") {
		t.Errorf("Errorf log should contain [ERROR]")
	}
	if !strings.Contains(buf.String(), "test error f") {
		t.Errorf("Errorf log should contain the formatted message")
	}
	buf.Reset()

	// Change level to Debug
	myLog.SetLevel(Debug)
	myLog.Debugf("test debug %s", "f")
	if !strings.Contains(buf.String(), "[DEBUG]") {
		t.Errorf("Debugf log should be printed when level is Debug")
	}
	if !strings.Contains(buf.String(), "test debug f") {
		t.Errorf("Debugf log should contain the formatted message")
	}
	buf.Reset()
}
