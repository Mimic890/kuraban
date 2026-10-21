package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger is a wrapper around zerolog.Logger
type Logger struct {
	zerolog.Logger
}

var (
	// Global logger instance
	globalLogger *Logger
)

// Config holds logger configuration
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// Init initializes the global logger with the given configuration
func Init(cfg Config) {
	var output io.Writer = os.Stdout

	// Set up console output for text format
	if cfg.Format == "text" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	// The logger itself accepts everything; the process-wide level decides,
	// so SetLevel can change verbosity at runtime from the web settings.
	SetLevel(cfg.Level)

	// Create logger
	logger := zerolog.New(output).
		Level(zerolog.TraceLevel).
		With().
		Timestamp().
		Caller().
		Logger()

	globalLogger = &Logger{logger}
	log.Logger = logger
}

// ParseLevel maps a level name to zerolog, defaulting to info.
func ParseLevel(name string) zerolog.Level {
	switch name {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// SetLevel changes the process-wide log level.
func SetLevel(name string) {
	zerolog.SetGlobalLevel(ParseLevel(name))
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		// Initialize with defaults if not initialized
		Init(Config{
			Level:  "info",
			Format: "text",
		})
	}
	return globalLogger
}

// Debug logs a debug message
func Debug() *zerolog.Event {
	return Get().Debug()
}

// Info logs an info message
func Info() *zerolog.Event {
	return Get().Info()
}

// Warn logs a warning message
func Warn() *zerolog.Event {
	return Get().Warn()
}

// Error logs an error message
func Error() *zerolog.Event {
	return Get().Error()
}

// Fatal logs a fatal message and exits
func Fatal() *zerolog.Event {
	return Get().Fatal()
}

// WithError creates a logger with an error field
func WithError(err error) *zerolog.Event {
	return Get().Error().Err(err)
}
