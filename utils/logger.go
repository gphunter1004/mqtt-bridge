package utils

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Logger
type Logger struct {
	*logrus.Logger
}

// NewLogger creates a new logger instance
func NewLogger(level string, logFile string) *Logger {
	logger := logrus.New()

	// Set log level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	// Set formatter
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})

	// Set output
	if logFile != "" {
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Errorf("Failed to create log directory: %v", err)
		}

		// Open log file
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.Errorf("Failed to open log file: %v", err)
		} else {
			logger.SetOutput(file)
		}
	}

	return &Logger{Logger: logger}
}

// Helper methods for consistent formatting

func (l *Logger) Infof(format string, args ...interface{}) {
	l.Logger.Infof(format, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Logger.Debugf(format, args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Logger.Warnf(format, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Logger.Errorf(format, args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Logger.Fatalf(format, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.Logger.Info(args...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.Logger.Debug(args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.Logger.Warn(args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.Logger.Error(args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.Logger.Fatal(args...)
}
