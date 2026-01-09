package input

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
	"github.com/yourname/lotty/pkg/types"
)

// PTYHandler handles PTY-based command execution and output capture
type PTYHandler struct {
	cmd    *exec.Cmd
	pty    *os.File
	output chan<- string
	logger *logrus.Logger
}

// NewPTYHandler creates a new PTYHandler
func NewPTYHandler(cmd *exec.Cmd, output chan<- string, logger *logrus.Logger) *PTYHandler {
	return &PTYHandler{
		cmd:    cmd,
		output: output,
		logger: logger,
	}
}

// Start starts the PTY handler
func (p *PTYHandler) Start() error {
	// Start command with PTY
	ptyFile, err := pty.Start(p.cmd)
	if err != nil {
		return err
	}
	p.pty = ptyFile

	// Set PTY size
	if err := pty.Setsize(ptyFile, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	}); err != nil {
		p.logger.Warnf("Failed to set PTY size: %v", err)
	}

	// Start reading from PTY
	go p.readFromPTY()

	return nil
}

// Wait waits for the command to complete
func (p *PTYHandler) Wait() error {
	return p.cmd.Wait()
}

// Close closes the PTY
func (p *PTYHandler) Close() error {
	if p.pty != nil {
		return p.pty.Close()
	}
	return nil
}

// readFromPTY reads output from the PTY
func (p *PTYHandler) readFromPTY() {
	buf := make([]byte, 1024)
	for {
		n, err := p.pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				p.logger.Errorf("Error reading from PTY: %v", err)
			}
			return
		}

		// Process output as lines
		text := string(buf[:n])
		lines := SplitLines(text)
		for _, line := range lines {
			if line != "" {
				p.output <- line
			}
		}
	}
}

// SplitLines splits text into lines
func SplitLines(text string) []string {
	lines := []string{}
	current := ""
	for _, ch := range text {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else if ch != '\r' {
			current += string(ch)
		}
	}
	// Add remaining text if exists
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// PipeHandler handles pipe-based command execution and output capture
type PipeHandler struct {
	cmd    *exec.Cmd
	output chan<- string
	logger *logrus.Logger
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// NewPipeHandler creates a new PipeHandler
func NewPipeHandler(cmd *exec.Cmd, output chan<- string, logger *logrus.Logger) *PipeHandler {
	return &PipeHandler{
		cmd:    cmd,
		output: output,
		logger: logger,
	}
}

// Start starts the pipe handler
func (p *PipeHandler) Start() error {
	var err error
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.stdout = stdout

	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		return err
	}
	p.stderr = stderr

	if err := p.cmd.Start(); err != nil {
		return err
	}

	// Read from stdout and stderr
	go p.readFromPipe(stdout, "stdout")
	go p.readFromPipe(stderr, "stderr")

	return nil
}

// Wait waits for the command to complete
func (p *PipeHandler) Wait() error {
	return p.cmd.Wait()
}

// Close closes the pipes
func (p *PipeHandler) Close() error {
	var err error
	if p.stdout != nil {
		if cerr := p.stdout.Close(); cerr != nil {
			err = cerr
		}
	}
	if p.stderr != nil {
		if cerr := p.stderr.Close(); cerr != nil {
			err = cerr
		}
	}
	return err
}

// readFromPipe reads output from a pipe
func (p *PipeHandler) readFromPipe(pipe io.Reader, name string) {
	reader := NewReader(pipe, p.output, p.logger)
	if err := reader.Start(); err != nil {
		p.logger.Errorf("Error reading from %s: %v", name, err)
	}
}

// CreateCommand creates a command to execute
func CreateCommand(cfg *types.Config) *exec.Cmd {
	return exec.Command(cfg.Command, cfg.Args...)
}

// RunCommand runs a command with the specified mode
func RunCommand(cfg *types.Config, output chan<- string, logger *logrus.Logger) (CommandHandler, error) {
	cmd := CreateCommand(cfg)

	if cfg.InputMode == "pty" {
		handler := NewPTYHandler(cmd, output, logger)
		if err := handler.Start(); err != nil {
			return nil, err
		}
		return handler, nil
	} else {
		handler := NewPipeHandler(cmd, output, logger)
		if err := handler.Start(); err != nil {
			return nil, err
		}
		return handler, nil
	}
}

// CommandHandler is an interface for command handlers
type CommandHandler interface {
	Start() error
	Wait() error
	Close() error
}

// SetupProcessGroup sets up the process group for proper signal handling
func SetupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// KillProcessGroup kills the entire process group
func KillProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// MonitorCommand monitors a command and restarts it if needed
func MonitorCommand(cfg *types.Config, output chan<- string, logger *logrus.Logger) <-chan error {
	errChan := make(chan error, 1)

	go func() {
		for {
			handler, err := RunCommand(cfg, output, logger)
			if err != nil {
				errChan <- err
				return
			}

			// Wait for command to finish
			err = handler.Wait()

			// Close handler
			if cerr := handler.Close(); cerr != nil {
				logger.Warnf("Error closing handler: %v", cerr)
			}

			// Send error
			if err != nil {
				errChan <- err
				return
			}

			// Sleep before restarting
			time.Sleep(1 * time.Second)
		}
	}()

	return errChan
}
