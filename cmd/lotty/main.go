package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
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
	cfgFile   string
	logLevel  string
	inputMode string
	echo      bool
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
	rootCmd.PersistentFlags().BoolVar(&echo, "no-echo", false, "suppress command output to stdout")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(cfgFile)
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
	if err := config.SetCommand(cfg, args[0], args[1:]...); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Setup logger
	logger := setupLogger(cfg.LogLevel)
	logger.WithFields(logrus.Fields{
		"input_mode":     cfg.InputMode,
		"loki_endpoint":  cfg.LokiEndpoint,
		"buffer_size":    cfg.BufferSize,
		"batch_interval": cfg.BatchInterval,
	}).Info("Starting lotty")

	// Create buffer
	buf := buffer.NewRingBuffer(cfg.BufferSize, logger)
	logger.Debugf("Created buffer with capacity: %d", buf.Capacity())

	// Create Loki client
	lokiClient := loki.NewClient(cfg.LokiEndpoint, 30*time.Second, logger)

	// Configure TLS
	if cfg.TLSSkipVerify {
		lokiClient.SetTLSConfig(cfg.TLSSkipVerify)
	}

	// Configure authentication
	if cfg.LokiUsername != "" || cfg.LokiPassword != "" {
		lokiClient.SetAuth(cfg.LokiUsername, cfg.LokiPassword)
		logger.Debugf("Basic Auth enabled")
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
	b.MonitorStats(cfg.StatsInterval)

	// Run command
	handler, err := input.RunCommand(cfg, inputChan, logger, !echo)
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}
	defer handler.Close()

	logger.WithFields(logrus.Fields{
		"command": cfg.Command,
		"args":    cfg.Args,
	}).Info("Running command")

	// Create a WaitGroup to track the input processor goroutine
	var inputWg sync.WaitGroup
	inputWg.Add(1)

	// Process input in goroutine
	go func() {
		defer inputWg.Done()
		for line := range inputChan {
			entry := buffer.CreateLogEntry(line)
			// Skip empty entries
			if entry.Line != "" {
				buf.Enqueue(entry)
			}
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
			logger.WithError(err).Error("Command exited with error")
		} else {
			logger.Info("Command completed successfully")
		}
		// Flush remaining logs after command completes
		b.Flush()

	case sig := <-sigChan:
		logger.WithField("signal", sig).Info("Received signal")
		logger.Info("Stopping lotty...")

		// Stop the command process gracefully
		if err := handler.Stop(); err != nil {
			logger.WithError(err).Warn("Failed to stop command gracefully")
		}

		// Wait for a short time to allow graceful shutdown
		shutdownDone := make(chan struct{}, 1)
		go func() {
			if err := handler.Wait(); err != nil {
				logger.WithError(err).Debug("Command exit status after stop")
			}
			close(shutdownDone)
		}()

		select {
		case <-shutdownDone:
			logger.Debug("Command stopped gracefully")
		case <-time.After(5 * time.Second):
			logger.Warn("Command did not stop gracefully, force close")
			handler.Close()
		}

		// Flush remaining logs before stopping
		b.Flush()
		logger.Debugf("Flushed remaining logs")
	}

	// Close input channel to signal goroutine to stop
	close(inputChan)

	// Wait for input processor goroutine to finish
	inputWg.Wait()

	// Stop batcher (this will trigger final flush and wait for completion)
	b.Stop()

	// Print final metrics
	metrics := b.GetMetrics()
	logger.WithFields(logrus.Fields{
		"messages_sent":    metrics.MessagesSent,
		"messages_dropped": metrics.MessagesDropped,
		"http_requests":    metrics.HTTPRequests,
		"http_failures":    metrics.HTTPFailures,
	}).Info("Final metrics")

	// Print buffer stats
	stats := buf.Stats()
	logger.WithFields(logrus.Fields{
		"enqueued": stats.Enqueued,
		"dequeued": stats.Dequeued,
		"dropped":  stats.Dropped,
	}).Debug("Buffer stats")

	logger.Info("Lotty stopped")

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
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05",
	})

	return logger
}
