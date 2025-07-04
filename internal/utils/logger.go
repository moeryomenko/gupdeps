package utils

import (
	"fmt"
	"log"
	"os"
)

// Logger provides enhanced logging capabilities
type Logger struct {
	info    *log.Logger
	warn    *log.Logger
	error   *log.Logger
	success *log.Logger
	verbose bool
}

// NewLogger creates a new logger
func NewLogger(verbose bool) *Logger {
	return &Logger{
		info:    log.New(os.Stdout, "🔍 INFO: ", log.LstdFlags),
		warn:    log.New(os.Stdout, "⚠️ WARN: ", log.LstdFlags),
		error:   log.New(os.Stderr, "❌ ERROR: ", log.LstdFlags),
		success: log.New(os.Stdout, "✅ SUCCESS: ", log.LstdFlags),
		verbose: verbose,
	}
}

// Info logs informational messages
func (l *Logger) Info(format string, v ...any) {
	if l.verbose {
		l.info.Printf(format, v...)
	}
}

// Warn logs warning messages
func (l *Logger) Warn(format string, v ...any) {
	l.warn.Printf(format, v...)
}

// Error logs error messages
func (l *Logger) Error(format string, v ...any) {
	l.error.Printf(format, v...)
}

// Success logs success messages
func (l *Logger) Success(format string, v ...any) {
	l.success.Printf(format, v...)
}

// Print outputs a message to stdout without any prefix
func (l *Logger) Print(format string, v ...any) {
	fmt.Printf(format+"\n", v...)
}
