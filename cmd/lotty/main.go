package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yourname/lotty/internal/batcher"
	"github.com/yourname/lotty/internal/buffer"
	"github.com/yourname/lotty/internal/config"
	"github.com/yourname/lotty/internal/input"
	"github.com/yourname/lotty/internal/loki"
)

var (
	cfgFile    string
	logLevel   string
	inputMode  string
	quiet      bool
)

var rootCmd = &cobra.Command{
	Use:   "lotty",
	Short: "A tool to send command output to Grafana Loki",
	Long: `Lotty is a tool that captures command output and sends it to Grafana Loki
via HTTP API. It supports PTY mode, in-memory buffering, and batch sending.`,
	Example: `  # Send ping output to Loki
  lotty -- ping google.com

  # Use pipe mode
  lotty --input-mode pipe -- tail -f /var/log/app.log`,
	RunE: run,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.lotty.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&inputMode, "input-mode", "pty", "input mode (pty, pipe)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress command output to stdout")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override log level from command line
	if logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Override input mode from command line
	if inputMode != "" {
		cfg.InputMode = inputMode
	}

	// Check if command is provided
	if len(args) == 0 {
		return fmt.Errorf("command is required")
	}

	// Set command and arguments
	config.SetCommand(cfg, args[0], args[1:]...)

	// Setup logger
	logger := setupLogger(cfg.LogLevel)
	logger.Infof("Starting lotty with input mode: %s", cfg.InputMode)
	logger.Infof("Loki endpoint: %s", cfg.LokiEndpoint)
	logger.Infof("Buffer size: %d", cfg.BufferSize)
	logger.Infof("Batch interval: %v", cfg.BatchInterval)

	// Create buffer
	buf := buffer.NewRingBuffer(cfg.BufferSize, logger)
	logger.Infof("Created buffer with capacity: %d", buf.Capacity())

	// Create Loki client
	lokiClient := loki.NewClient(cfg.LokiEndpoint, 30*time.Second, logger)
	if cfg.LokiUsername != "" || cfg.LokiPassword != "" {
		lokiClient.SetAuth(cfg.LokiUsername, cfg.LokiPassword)
		logger.Infof("Basic Auth enabled")
	}

	// Create batcher
	b := batcher.NewBatcher(
		buf,
		lokiClient,
		cfg.LokiLabels,
		cfg.BatchInterval,
		cfg.BatchSize,
		cfg.RetryCount,
		logger,
	)

	// Create input channel
	inputChan := make(chan string, 100)

	// Start batcher
	b.Start()

	// Start stats monitor
	b.MonitorStats(30 * time.Second)

	// Run command
	handler, err := input.RunCommand(cfg, inputChan, logger, quiet)
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}
	defer handler.Close()

	logger.Infof("Running command: %s %v", cfg.Command, cfg.Args)

	// Process input in goroutine
	go func() {
		for line := range inputChan {
			entry := buffer.CreateLogEntry(line)
			buf.Enqueue(entry)
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for command to complete or signal
	errChan := make(chan error, 1)
	go func() {
		errChan <- handler.Wait()
	}()

	select {
	case err := <-errChan:
		if err != nil {
			logger.Errorf("Command exited with error: %v", err)
		} else {
			logger.Infof("Command completed successfully")
		}
		// Flush remaining logs after command completes
		b.Flush()

	case sig := <-sigChan:
		logger.Infof("Received signal: %v", sig)
		logger.Infof("Stopping lotty...")

		// Flush remaining logs before stopping
		b.Flush()
		logger.Infof("Flushed remaining logs")
	}

	// Wait a bit for the flush to complete
	time.Sleep(200 * time.Millisecond)

	// Stop batcher (this will trigger final flush and wait for completion)
	b.Stop()

	// Print final metrics
	metrics := b.GetMetrics()
	logger.Infof("Final metrics:")
	logger.Infof("  Messages sent: %d", metrics.MessagesSent)
	logger.Infof("  Messages dropped: %d", metrics.MessagesDropped)
	logger.Infof("  HTTP requests: %d", metrics.HTTPRequests)
	logger.Infof("  HTTP failures: %d", metrics.HTTPFailures)

	// Print buffer stats
	stats := buf.Stats()
	logger.Infof("Buffer stats:")
	logger.Infof("  Enqueued: %d", stats.Enqueued)
	logger.Infof("  Dequeued: %d", stats.Dequeued)
	logger.Infof("  Dropped: %d", stats.Dropped)

	logger.Infof("Lotty stopped")

	return nil
}

func setupLogger(level string) *logrus.Logger {
	logger := logrus.New()

	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		TimestampFormat: "2006-01-02T15:04:05",
	})

	return logger
}
