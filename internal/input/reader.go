package input

import (
	"bufio"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

// Reader reads from standard input and sends log lines to a channel
type Reader struct {
	reader io.Reader
	output chan<- string
	logger *logrus.Logger
}

// NewReader creates a new Reader
func NewReader(reader io.Reader, output chan<- string, logger *logrus.Logger) *Reader {
	return &Reader{
		reader: reader,
		output: output,
		logger: logger,
	}
}

// Start begins reading from the reader
func (r *Reader) Start() error {
	scanner := bufio.NewScanner(r.reader)
	for scanner.Scan() {
		line := scanner.Text()
		r.output <- line
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
