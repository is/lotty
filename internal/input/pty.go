package input

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
	"github.com/yourname/lotty/pkg/types"
)

// PTYHandler handles PTY-based command execution and output capture
type PTYHandler struct {
	cmd     *exec.Cmd
	pty     *os.File
	output  chan<- string
	logger  *logrus.Logger
	echo    bool
	ptyRows int
	ptyCols int
}

// NewPTYHandler creates a new PTYHandler
func NewPTYHandler(cmd *exec.Cmd, output chan<- string, logger *logrus.Logger, echo bool) *PTYHandler {
	return &PTYHandler{
		cmd:     cmd,
		output:  output,
		logger:  logger,
		echo:    echo,
		ptyRows: 24,
		ptyCols: 80,
	}
}

// SetPTYSize sets the PTY terminal size
func (p *PTYHandler) SetPTYSize(rows, cols int) {
	p.ptyRows = rows
	p.ptyCols = cols
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
		Rows: uint16(p.ptyRows),
		Cols: uint16(p.ptyCols),
	}); err != nil {
		p.logger.Debugf("Failed to set PTY size: %v", err)
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

// Stop gracefully stops the command by sending SIGTERM
func (p *PTYHandler) Stop() error {
	if p.cmd.Process != nil {
		p.logger.Info("Sending SIGTERM to command process")
		if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			p.logger.WithError(err).Warn("Failed to send SIGTERM, trying SIGKILL")
			// If SIGTERM fails, try SIGKILL
			return p.cmd.Process.Kill()
		}
	}
	return nil
}

// readFromPTY reads output from the PTY
func (p *PTYHandler) readFromPTY() {
	buf := make([]byte, 1024)
	for {
		n, err := p.pty.Read(buf)
		if err != nil {
			// PTY read errors are common when the command completes
			// These are not real errors, just normal shutdown
			if err == io.EOF {
				// Normal EOF when command completes
				p.logger.Debugf("PTY closed (command completed)")
			} else if isExpectedPTYError(err) {
				// Other expected PTY errors (e.g., broken pipe, I/O errors)
				p.logger.Debugf("PTY closed with expected error: %v", err)
			} else {
				// Unexpected error that should be logged
				p.logger.Errorf("Error reading from PTY: %v", err)
			}
			return
		}

		// Process output as lines
		text := string(buf[:n])
		lines := SplitLines(text)
		for _, line := range lines {
			if line != "" {
				// Send to output channel (for Loki)
				select {
				case p.output <- line:
					// Send successful
				default:
					// Channel closed or full, skip this line
					p.logger.Debugf("Output channel closed or full, skipping line")
					return
					}
					// Also print to stdout if echo mode is enabled
					if p.echo {
						fmt.Println(line)
					}
			}
		}
	}
}

// isExpectedPTYError checks if an error is an expected PTY shutdown error
func isExpectedPTYError(err error) bool {
	if err == nil {
		return false
	}
	// Check for timeout errors
	if os.IsTimeout(err) {
		return true
	}
	// Check for common PTY shutdown errors
	errStr := err.Error()
	// EIO - Input/output error (common with PTY on Linux)
	// EPIPE - Broken pipe (when process terminates)
	// These are expected during normal shutdown
	return strings.Contains(errStr, "input/output error") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "operation not permitted")
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
	echo   bool
}

// NewPipeHandler creates a new PipeHandler
func NewPipeHandler(cmd *exec.Cmd, output chan<- string, logger *logrus.Logger, echo bool) *PipeHandler {
	return &PipeHandler{
		cmd:    cmd,
		output: output,
		logger: logger,
		echo:   echo,
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

// Stop gracefully stops the command by sending SIGTERM
func (p *PipeHandler) Stop() error {
	if p.cmd.Process != nil {
		p.logger.Info("Sending SIGTERM to command process")
		if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			p.logger.WithError(err).Warn("Failed to send SIGTERM, trying SIGKILL")
			// If SIGTERM fails, try SIGKILL
			return p.cmd.Process.Kill()
		}
	}
	return nil
}

// readFromPipe reads output from a pipe
func (p *PipeHandler) readFromPipe(pipe io.Reader, name string) {
	reader := NewReader(pipe, p.output, p.logger, p.echo)
	if err := reader.Start(); err != nil {
		p.logger.Errorf("Error reading from %s: %v", name, err)
	}
}

// CreateCommand creates a command to execute
func CreateCommand(cfg *types.Config) *exec.Cmd {
	return exec.Command(cfg.Command, cfg.Args...)
}

// RunCommand runs a command with the specified mode
func RunCommand(cfg *types.Config, output chan<- string, logger *logrus.Logger, echo bool) (CommandHandler, error) {
	cmd := CreateCommand(cfg)

	if cfg.InputMode == "pty" {
		handler := NewPTYHandler(cmd, output, logger, echo)
		handler.SetPTYSize(cfg.PTYRows, cfg.PTYCols)
		if err := handler.Start(); err != nil {
			return nil, err
		}
		return handler, nil
	} else {
		handler := NewPipeHandler(cmd, output, logger, echo)
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
	Stop() error
}
