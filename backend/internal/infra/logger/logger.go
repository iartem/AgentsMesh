package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
	FilePath   string // path to log file, empty means stdout only
	MaxSizeMB  int    // max size in MB before rotation (0 = no limit)
	MaxBackups int    // max number of backup files to keep
}

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	file   *os.File
	config Config
}

// New creates a new logger with the given configuration
func New(cfg Config) (*Logger, error) {
	var writers []io.Writer

	// Always write to stdout
	writers = append(writers, os.Stdout)

	var logFile *os.File

	// Optionally write to file
	if cfg.FilePath != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open log file (append mode)
		f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logFile = f
		writers = append(writers, f)
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
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(multiWriter, opts)
	} else {
		handler = slog.NewTextHandler(multiWriter, opts)
	}

	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		file:   logFile,
		config: cfg,
	}, nil
}

// Close closes the log file if open
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// RotateIfNeeded rotates the log file if it exceeds the max size
func (l *Logger) RotateIfNeeded() error {
	if l.file == nil || l.config.MaxSizeMB <= 0 {
		return nil
	}

	info, err := l.file.Stat()
	if err != nil {
		return err
	}

	maxBytes := int64(l.config.MaxSizeMB) * 1024 * 1024
	if info.Size() < maxBytes {
		return nil
	}

	return l.rotate()
}

// rotate performs log rotation
func (l *Logger) rotate() error {
	if l.file == nil {
		return nil
	}

	// Close current file
	if err := l.file.Close(); err != nil {
		return err
	}

	// Rename current log file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", l.config.FilePath, timestamp)
	if err := os.Rename(l.config.FilePath, backupPath); err != nil {
		return err
	}

	// Open new log file
	f, err := os.OpenFile(l.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.file = f

	// Clean up old backup files
	l.cleanupBackups()

	return nil
}

// cleanupBackups removes old backup files beyond MaxBackups
func (l *Logger) cleanupBackups() {
	if l.config.MaxBackups <= 0 {
		return
	}

	dir := filepath.Dir(l.config.FilePath)
	base := filepath.Base(l.config.FilePath)
	pattern := base + ".*"

	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return
	}

	// Remove oldest files if we have too many
	if len(matches) > l.config.MaxBackups {
		// Files are sorted by name which includes timestamp, oldest first
		for i := 0; i < len(matches)-l.config.MaxBackups; i++ {
			os.Remove(matches[i])
		}
	}
}

// parseLevel converts string level to slog.Level
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

// SetDefault sets this logger as the default slog logger
func (l *Logger) SetDefault() {
	slog.SetDefault(l.Logger)
}

// Gin returns an io.Writer for Gin's logging middleware
func (l *Logger) GinWriter() io.Writer {
	if l.file != nil {
		return io.MultiWriter(os.Stdout, l.file)
	}
	return os.Stdout
}
