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
	output chan<- string
	logger *logrus.Logger
	quiet  bool
}

// NewReader creates a new Reader
func NewReader(reader io.Reader, output chan<- string, logger *logrus.Logger) *Reader {
	return &Reader{
		reader: reader,
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
	scanner := bufio.NewScanner(r.reader)
	for scanner.Scan() {
		line := scanner.Text()
		// Send to output channel (for Loki)
		r.output <- line
		// Also print to stdout unless in quiet mode
		if !r.quiet {
			fmt.Println(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// ReadLine reads a single line from the reader with a timeout
func (r *Reader) ReadLine(timeout time.Duration) (string, error) {
	scanner := bufio.NewScanner(r.reader)
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		if scanner.Scan() {
			resultChan <- scanner.Text()
		} else {
			errChan <- scanner.Err()
		}
	}()

	select {
	case line := <-resultChan:
		return line, nil
	case err := <-errChan:
		return "", err
	case <-time.After(timeout):
		return "", io.EOF
	}
}
