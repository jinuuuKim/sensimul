package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

// Logger wraps zerolog.Logger to provide app-level lifecycle methods.
type Logger struct {
	zerolog.Logger
}

// Init initializes structured JSON logging to stderr.
// Returns the logger instance for use in the application.
func Init() Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFieldName = "timestamp"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "message"

	log = zerolog.New(os.Stderr).
		With().
		Timestamp().
		Str("service", "sensimul").
		Logger()

	return Logger{Logger: log}
}

// Get returns the initialized logger instance.
func Get() Logger {
	return Logger{Logger: log}
}

// Close syncs the log buffer. Called via defer in main.
func (l Logger) Close() {
}

// NewLogger creates a new logger with additional context fields.
func NewLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// DevFormat returns a console-friendly logger for development.
func DevFormat() zerolog.Logger {
	console := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	return zerolog.New(console).
		With().
		Timestamp().
		Str("service", "sensimul").
		Logger()
}
