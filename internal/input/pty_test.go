package input

import (
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourname/lotty/pkg/types"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "single line",
			text:     "line1",
			expected: []string{"line1"},
		},
		{
			name:     "multiple lines",
			text:     "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "lines with CRLF",
			text:     "line1\r\nline2\r\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "trailing newline",
			text:     "line1\nline2\n",
			expected: []string{"line1", "line2"},
		},
		{
			name:     "empty lines",
			text:     "line1\n\nline2",
			expected: []string{"line1", "", "line2"},
		},
		{
			name:     "carriage return stripped with newline",
			text:     "line1\r\nline2",
			expected: []string{"line1", "line2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitLines(tt.text)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Line %d: expected '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestCreateCommand(t *testing.T) {
	cfg := &types.Config{
		Command: "echo",
		Args:    []string{"hello", "world"},
	}

	cmd := CreateCommand(cfg)

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	if cmd.Path != "echo" {
		t.Errorf("Expected path 'echo', got '%s'", cmd.Path)
	}

	if len(cmd.Args) != 3 {
		t.Errorf("Expected 3 args, got %d", len(cmd.Args))
	}

	if cmd.Args[1] != "hello" || cmd.Args[2] != "world" {
		t.Errorf("Expected args [echo hello world], got %v", cmd.Args)
	}
}

func TestReader_ReadLine_Timeout(t *testing.T) {
	logger := logrus.New()
	output := make(chan string, 10)

	reader := NewReader(strings.NewReader("line1\nline2\n"), output, logger)

	line, err := reader.ReadLine(1 * time.Second)
	if err != nil {
		t.Errorf("ReadLine() error = %v", err)
	}

	if line != "line1" {
		t.Errorf("Expected 'line1', got '%s'", line)
	}
}

func TestPTYHandler(t *testing.T) {
	cfg := &types.Config{
		Command: "echo",
		Args:    []string{"test"},
	}

	logger := logrus.New()
	output := make(chan string, 10)

	handler := NewPTYHandler(CreateCommand(cfg), output, logger)

	// Note: PTY tests may not work in all environments
	// This is a basic structure test
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}

	if handler.cmd == nil {
		t.Error("Expected command to be set")
	}
}

func TestPipeHandler(t *testing.T) {
	cfg := &types.Config{
		Command: "echo",
		Args:    []string{"test"},
	}

	logger := logrus.New()
	output := make(chan string, 10)

	handler := NewPipeHandler(CreateCommand(cfg), output, logger)

	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}

	if handler.cmd == nil {
		t.Error("Expected command to be set")
	}

	// Test starting the handler
	err := handler.Start()
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	// Wait for command to complete
	err = handler.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	// Close the handler
	err = handler.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestRunCommand_PipeMode(t *testing.T) {
	cfg := &types.Config{
		Command:   "echo",
		Args:      []string{"test output"},
		InputMode: "pipe",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	output := make(chan string, 10)

	handler, err := RunCommand(cfg, output, logger)
	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}

	// Wait for command to complete
	err = handler.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	err = handler.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestCommandHandler_Interface(t *testing.T) {
	cfg := &types.Config{
		Command:   "echo",
		Args:      []string{"test"},
		InputMode: "pipe",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	output := make(chan string, 10)

	handler, err := RunCommand(cfg, output, logger)
	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}

	// Test that handler implements CommandHandler interface
	var ch CommandHandler = handler

	err = ch.Wait()
	if err != nil {
		t.Errorf("CommandHandler.Wait() error = %v", err)
	}

	err = ch.Close()
	if err != nil {
		t.Errorf("CommandHandler.Close() error = %v", err)
	}
}

func TestMonitorCommand(t *testing.T) {
	cfg := &types.Config{
		Command:   "echo",
		Args:      []string{"test"},
		InputMode: "pipe",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	output := make(chan string, 10)

	errChan := MonitorCommand(cfg, output, logger)

	// Wait for error or timeout
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("MonitorCommand() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("MonitorCommand() timeout")
	}
}
