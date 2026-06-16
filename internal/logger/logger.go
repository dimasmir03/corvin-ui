package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

const (
	defaultLevel  = "info"
	defaultFormat = "text"
)

type Logger struct {
	logger *slog.Logger
}

var (
	mu            sync.RWMutex
	defaultLogger = newLogger(os.Stdout, defaultLevel, defaultFormat)
)

func New(level string, format string) *Logger {
	return newLogger(os.Stdout, level, format)
}

func NewLogger(dest string, level string) *Logger {
	writer := writerForDest(dest)
	log := newLogger(writer, level, defaultFormat)
	SetDefault(log)
	return log
}

func SetDefault(log *Logger) {
	if log == nil {
		return
	}
	mu.Lock()
	defaultLogger = log
	mu.Unlock()
}

func Default() *Logger {
	mu.RLock()
	log := defaultLogger
	mu.RUnlock()
	return log
}

func With(args ...any) *Logger {
	return Default().With(args...)
}

func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

func Error(msg string, err error, args ...any) {
	Default().Error(msg, err, args...)
}

func (l *Logger) With(args ...any) *Logger {
	if l == nil || l.logger == nil {
		return Default().With(args...)
	}
	return &Logger{logger: l.logger.With(args...)}
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(slog.LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(slog.LevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(slog.LevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err)
	}
	l.log(slog.LevelError, msg, args...)
}

func (l *Logger) log(level slog.Level, msg string, args ...any) {
	if l == nil || l.logger == nil {
		Default().log(level, msg, args...)
		return
	}
	l.logger.Log(context.Background(), level, msg, args...)
}

func newLogger(writer io.Writer, level string, format string) *Logger {
	if writer == nil {
		writer = os.Stdout
	}

	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: slogLevel}
	var handler slog.Handler
	if strings.EqualFold(strings.TrimSpace(format), "json") {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	return &Logger{logger: slog.New(handler)}
}

func writerForDest(dest string) io.Writer {
	switch strings.ToLower(strings.TrimSpace(dest)) {
	case "file":
		return logFileWriter()
	case "all":
		return io.MultiWriter(logFileWriter(), os.Stdout)
	default:
		return os.Stdout
	}
}

func logFileWriter() io.Writer {
	file, err := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return os.Stdout
	}
	return file
}
