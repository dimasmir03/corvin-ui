package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	slogger "log/slog"
)

const (
	defaultLevel  = "info"
	defaultFormat = "text"
)

type Logger struct {
	logger *slogger.Logger
	writer io.Writer
}

var (
	mu            sync.RWMutex
	defaultLogger = newLogger(os.Stdout, defaultLevel, defaultFormat)
)

func New(level string, format string) *Logger {
	return newLogger(os.Stdout, level, format)
}

func NewWithWriter(writer io.Writer, level string, format string) *Logger {
	return newLogger(writer, level, format)
}

func Configure(writer io.Writer, level string, format string) *Logger {
	log := newLogger(writer, level, format)
	SetDefault(log)
	return log
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

func Writer() io.Writer {
	return Default().Writer()
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

func Print(args ...any) {
	Default().Print(args...)
}

func Println(args ...any) {
	Default().Println(args...)
}

func Printf(format string, args ...any) {
	Default().Printf(format, args...)
}

func Fatal(args ...any) {
	Default().Print(args...)
	os.Exit(1)
}

func Fatalf(format string, args ...any) {
	Default().Printf(format, args...)
	os.Exit(1)
}

func Panic(args ...any) {
	msg := fmt.Sprint(args...)
	Default().Info(msg)
	panic(msg)
}

func Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	Default().Info(msg)
	panic(msg)
}

func (l *Logger) With(args ...any) *Logger {
	if l == nil || l.logger == nil {
		return Default().With(args...)
	}
	return &Logger{logger: l.logger.With(args...), writer: l.writer}
}

func (l *Logger) Writer() io.Writer {
	if l == nil || l.writer == nil {
		return os.Stdout
	}
	return l.writer
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(slogger.LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(slogger.LevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(slogger.LevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err)
	}
	l.log(slogger.LevelError, msg, args...)
}

func (l *Logger) Print(args ...any) {
	l.Info(fmt.Sprint(args...))
}

func (l *Logger) Println(args ...any) {
	l.Info(fmt.Sprintln(args...))
}

func (l *Logger) Printf(format string, args ...any) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l *Logger) log(level slogger.Level, msg string, args ...any) {
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

	var slogLevel slogger.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slogger.LevelDebug
	case "warn", "warning":
		slogLevel = slogger.LevelWarn
	case "error":
		slogLevel = slogger.LevelError
	default:
		slogLevel = slogger.LevelInfo
	}

	opts := &slogger.HandlerOptions{Level: slogLevel}
	var handler slogger.Handler
	if strings.EqualFold(strings.TrimSpace(format), "json") {
		handler = slogger.NewJSONHandler(writer, opts)
	} else {
		handler = slogger.NewTextHandler(writer, opts)
	}

	return &Logger{logger: slogger.New(handler), writer: writer}
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
	file, err := os.OpenFile("logger.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return os.Stdout
	}
	return file
}
