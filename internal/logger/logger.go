package logger

import (
	"log"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
	Fatal(v ...any)
	Fatalf(format string, v ...any)
}

type AppLogger struct {
	*log.Logger
}

func New(logFilePath string) *AppLogger {
	l := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	stdLogger := log.New(l, "", log.LstdFlags|log.Lshortfile)
	return &AppLogger{Logger: stdLogger}
}
