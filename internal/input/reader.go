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
	reader  io.Reader
	scanner *bufio.Scanner
	output  chan<- string
	logger  *logrus.Logger
	echo    bool
	done    <-chan struct{}
}

// NewReader creates a new Reader
func NewReader(reader io.Reader, output chan<- string, logger *logrus.Logger, echo bool, done <-chan struct{}) *Reader {
	return &Reader{
		reader:  reader,
		scanner: bufio.NewScanner(reader),
		output:  output,
		logger:  logger,
		echo:    echo,
		done:    done,
	}
}

// Start begins reading from the reader
func (r *Reader) Start() error {
	for {
		select {
		case <-r.done:
			// Signal to stop reading
			return nil
		default:
			// Continue reading
		}

		if !r.scanner.Scan() {
			break
		}
		line := r.scanner.Text()
		// Send to output channel (for Loki)
		select {
		case r.output <- line:
			// Send successful
		case <-r.done:
			// Signal to stop reading
			return nil
		default:
			// Channel full, log warning but continue
			r.logger.Warn("Output channel full, skipping line (data may be lost)")
		}
		// Also print to stdout if echo mode is enabled
		if r.echo {
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
