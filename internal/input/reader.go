package input

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

// Reader reads from standard input and sends log lines to a channel
type Reader struct {
	reader io.Reader
	scanner *bufio.Scanner
	output chan<- string
	logger *logrus.Logger
	quiet  bool
}

// NewReader creates a new Reader
func NewReader(reader io.Reader, output chan<- string, logger *logrus.Logger) *Reader {
	return &Reader{
		reader: reader,
		scanner: bufio.NewScanner(reader),
		output: output,
		logger: logger,
	}
}

// SetQuiet sets the quiet mode for the reader
func (r *Reader) SetQuiet(quiet bool) {
	r.quiet = quiet
}

// Start begins reading from the reader
func (r *Reader) Start() error {
	for r.scanner.Scan() {
		line := r.scanner.Text()
		// Send to output channel (for Loki)
		select {
		case r.output <- line:
			// Send successful
		default:
			// Channel closed or full, skip this line
			return fmt.Errorf("output channel closed or full")
		}
		// Also print to stdout unless in quiet mode
		if !r.quiet {
			fmt.Println(line)
		}
	}
	if err := r.scanner.Err(); err != nil {
		return err
	}
	return nil
}

// ReadLine reads a single line from the reader with a timeout
func (r *Reader) ReadLine(timeout time.Duration) (string, error) {
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		if r.scanner.Scan() {
			resultChan <- r.scanner.Text()
		} else {
			errChan <- r.scanner.Err()
		}
	}()

	select {
	case line := <-resultChan:
		return line, nil
	case err := <-errChan:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("read timeout")
	}
}
