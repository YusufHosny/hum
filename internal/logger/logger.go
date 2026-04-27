package logger

import (
	"log"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger is the interface that components use to write logs
type Logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
	Fatal(v ...any)
	Fatalf(format string, v ...any)
}

// AppLogger is the concrete implementation wrapping standard log
type AppLogger struct {
	*log.Logger
}

// New creates a new rotating file logger
func New(logFilePath string) *AppLogger {
	l := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	}

	stdLogger := log.New(l, "", log.LstdFlags|log.Lshortfile)
	return &AppLogger{Logger: stdLogger}
}
