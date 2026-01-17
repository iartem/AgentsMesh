// Package logger provides structured logging for the Runner using slog.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultMaxFileSize is the default maximum log file size (10MB)
	DefaultMaxFileSize = 10 * 1024 * 1024
	// DefaultMaxBackups is the default number of backup files to keep
	DefaultMaxBackups = 3
)

// Config holds logger configuration.
type Config struct {
	Level       string // debug, info, warn, error
	FilePath    string // path to log file, empty means stderr only
	Format      string // json, text (default: text)
	MaxFileSize int64  // max file size in bytes before rotation (default: 10MB)
	MaxBackups  int    // max number of backup files to keep (default: 3)
}

// Logger wraps slog.Logger with additional functionality.
type Logger struct {
	*slog.Logger
	writer *rotatingWriter
	config Config
}

// rotatingWriter implements io.Writer with log rotation support.
type rotatingWriter struct {
	filePath    string
	maxSize     int64
	maxBackups  int
	currentSize int64
	file        *os.File
	mu          sync.Mutex
}

func newRotatingWriter(filePath string, maxSize int64, maxBackups int) (*rotatingWriter, error) {
	rw := &rotatingWriter{
		filePath:   filePath,
		maxSize:    maxSize,
		maxBackups: maxBackups,
	}

	if err := rw.openFile(); err != nil {
		return nil, err
	}

	return rw, nil
}

func (rw *rotatingWriter) openFile() error {
	// Ensure directory exists
	dir := filepath.Dir(rw.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file (append mode)
	f, err := os.OpenFile(rw.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	rw.file = f
	rw.currentSize = info.Size()
	return nil
}

func (rw *rotatingWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Check if rotation is needed
	if rw.currentSize+int64(len(p)) > rw.maxSize {
		if err := rw.rotate(); err != nil {
			// Log rotation failed, but continue writing to current file
			// to avoid losing log data
			fmt.Fprintf(os.Stderr, "log rotation failed: %v\n", err)
		}
	}

	n, err = rw.file.Write(p)
	rw.currentSize += int64(n)
	return n, err
}

func (rw *rotatingWriter) rotate() error {
	// Close current file
	if rw.file != nil {
		rw.file.Close()
	}

	// Remove oldest backup if we have too many
	for i := rw.maxBackups - 1; i >= 0; i-- {
		oldPath := rw.backupPath(i)
		newPath := rw.backupPath(i + 1)

		if i == rw.maxBackups-1 {
			// Remove the oldest backup
			os.Remove(oldPath)
		} else {
			// Rename backup.N to backup.N+1
			if _, err := os.Stat(oldPath); err == nil {
				os.Rename(oldPath, newPath)
			}
		}
	}

	// Rename current log to backup.0
	if _, err := os.Stat(rw.filePath); err == nil {
		os.Rename(rw.filePath, rw.backupPath(0))
	}

	// Open new file
	return rw.openFile()
}

func (rw *rotatingWriter) backupPath(index int) string {
	return fmt.Sprintf("%s.%d", rw.filePath, index)
}

func (rw *rotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file != nil {
		return rw.file.Close()
	}
	return nil
}

var (
	defaultLogger *Logger
	mu            sync.RWMutex
)

// Init initializes the global logger with the given configuration.
func Init(cfg Config) error {
	logger, err := New(cfg)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	// Close previous logger if exists
	if defaultLogger != nil && defaultLogger.writer != nil {
		defaultLogger.writer.Close()
	}

	defaultLogger = logger
	slog.SetDefault(logger.Logger)
	return nil
}

// New creates a new logger with the given configuration.
func New(cfg Config) (*Logger, error) {
	var writers []io.Writer

	// Always write to stderr (stdout reserved for user interaction)
	writers = append(writers, os.Stderr)

	var rotWriter *rotatingWriter

	// Optionally write to file with rotation
	if cfg.FilePath != "" {
		maxSize := cfg.MaxFileSize
		if maxSize <= 0 {
			maxSize = DefaultMaxFileSize
		}

		maxBackups := cfg.MaxBackups
		if maxBackups <= 0 {
			maxBackups = DefaultMaxBackups
		}

		rw, err := newRotatingWriter(cfg.FilePath, maxSize, maxBackups)
		if err != nil {
			return nil, err
		}
		rotWriter = rw
		writers = append(writers, rw)
	}

	// Create multi-writer
	multiWriter := io.MultiWriter(writers...)

	// Parse log level
	level := parseLevel(cfg.Level)

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format time as short format for text output
			if a.Key == slog.TimeKey && cfg.Format != "json" {
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String(slog.TimeKey, t.Format("15:04:05.000"))
				}
			}
			return a
		},
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(multiWriter, opts)
	} else {
		handler = slog.NewTextHandler(multiWriter, opts)
	}

	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		writer: rotWriter,
		config: cfg,
	}, nil
}

// Close closes the log file if open.
func (l *Logger) Close() error {
	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

// parseLevel converts string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Close closes the default logger.
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if defaultLogger != nil && defaultLogger.writer != nil {
		return defaultLogger.writer.Close()
	}
	return nil
}

// Default returns the default logger.
func Default() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()

	if defaultLogger != nil {
		return defaultLogger.Logger
	}
	return slog.Default()
}
