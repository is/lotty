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
	done    chan struct{}
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
		done:    make(chan struct{}),
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

	// Set PTY size (failure is non-critical, but log it)
	if err := pty.Setsize(ptyFile, &pty.Winsize{
		Rows: uint16(p.ptyRows),
		Cols: uint16(p.ptyCols),
	}); err != nil {
		p.logger.Debugf("Failed to set PTY size: %v", err)
	}

	// Start reading from PTY in a goroutine
	go p.readFromPTY()

	return nil
}

// Wait waits for the command to complete
func (p *PTYHandler) Wait() error {
	return p.cmd.Wait()
}

// Close closes the PTY and signals the read goroutine to stop
func (p *PTYHandler) Close() error {
	// Signal the read goroutine to stop
	close(p.done)
	
	if p.pty != nil {
		if err := p.pty.Close(); err != nil {
			return err
		}
		p.pty = nil
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
	defer func() {
		// Ensure PTY is closed when goroutine exits
		if p.pty != nil {
			p.pty.Close()
			p.pty = nil
		}
	}()

	buf := make([]byte, 1024)
	for {
		select {
		case <-p.done:
			// Signal to stop reading
			return
		default:
			// Continue reading
		}

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
					// Channel full, log warning but continue
					p.logger.Warn("Output channel full, skipping line (data may be lost)")
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

// SplitLines splits text into lines efficiently using strings.Builder
func SplitLines(text string) []string {
	lines := make([]string, 0)
	var builder strings.Builder

	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\n' {
			lines = append(lines, builder.String())
			builder.Reset()
		} else if ch != '\r' {
			builder.WriteByte(ch)
		}
	}

	// Add remaining text if exists
	if builder.Len() > 0 {
		lines = append(lines, builder.String())
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
	done   chan struct{}
}

// NewPipeHandler creates a new PipeHandler
func NewPipeHandler(cmd *exec.Cmd, output chan<- string, logger *logrus.Logger, echo bool) *PipeHandler {
	return &PipeHandler{
		cmd:    cmd,
		output: output,
		logger: logger,
		echo:   echo,
		done:   make(chan struct{}),
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

// Close closes the pipes and signals goroutines to stop
func (p *PipeHandler) Close() error {
	// Signal the read goroutines to stop
	close(p.done)

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
	reader := NewReader(pipe, p.output, p.logger, p.echo, p.done)
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
