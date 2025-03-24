package utils

import (
	"strings"
	"sync"

	"github.com/mattn/go-colorable"
	"github.com/sirupsen/logrus"
)

var logColors = map[logrus.Level]string{
	logrus.DebugLevel:   "\033[36m", // Cyan
	logrus.InfoLevel:    "\033[32m", // Green
	logrus.WarnLevel:    "\033[33m", // Yellow
	logrus.ErrorLevel:   "\033[31m", // Red
	logrus.FatalLevel:   "\033[35m", // Magenta
	logrus.PanicLevel:   "\033[41m", // Bold Red (Background)
}

var (
	logger *logrus.Logger
	loggerOnce sync.Once
)

// Custom formatter to apply colors and alignment
type ColorFormatter struct {
	logrus.TextFormatter
}

// padRight ensures the log level string is at least 8 characters wide
func padRight(str string, length int) string {
	if len(str) >= length {
		return str
	}
	return str + strings.Repeat(" ", length-len(str))
}

// Format function applies colors and alignment
func (f *ColorFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	color, exists := logColors[entry.Level]
	if !exists {
		color = "\033[0m" // Default color
	}

	// Ensure log level is properly aligned to 8 characters
	levelStr := padRight(entry.Level.String(), 8)

	// Apply color to the entire log message, including timestamp, log level, and message
	output := []byte(color + entry.Time.Format("2006-01-02 15:04:05") + " | " +
		levelStr + "| " + entry.Message + "\033[0m\n")

	return output, nil
}

// SetupLogger initializes the logger with colors
func SetupLogger() *logrus.Logger {
	loggerOnce.Do(func() {
		logger = logrus.New()
		logger.SetOutput(colorable.NewColorableStdout()) // Enable color logging
		logger.SetFormatter(&ColorFormatter{})           // Use custom formatter
		logger.SetLevel(logrus.DebugLevel)               // Default log level
	})

	// Ensure logger is not nil before returning it
	if logger == nil {
		logger = logrus.New() // Fallback in case of unexpected nil
	}

	return logger
}
