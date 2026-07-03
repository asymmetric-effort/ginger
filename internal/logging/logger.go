package logging

import (
	"context"
	"io"
	"os"
)

type contextKeyType int

const loggerContextKey contextKeyType = 0

// Logger is the interface for structured logging.
type Logger interface {
	// Debug logs a message at DEBUG level.
	Debug(msg string, fields ...Field)
	// Info logs a message at INFO level.
	Info(msg string, fields ...Field)
	// Warn logs a message at WARN level.
	Warn(msg string, fields ...Field)
	// Error logs a message at ERROR level.
	Error(msg string, fields ...Field)
	// Fatal logs a message at FATAL level and calls os.Exit(1).
	Fatal(msg string, fields ...Field)
	// With returns a child logger with the given fields prepended.
	With(fields ...Field) Logger
	// Level returns the current atomic level.
	Level() *AtomicLevel
}

// Config configures a logger.
type Config struct {
	// Level is the minimum log level.
	Level Level
	// Output is the writer for log output. Defaults to os.Stderr.
	Output io.Writer
	// Format selects the encoder: "json" or "text".
	Format string
}

// New creates a new Logger from the given config.
func New(cfg Config) Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}
	level := NewAtomicLevel(cfg.Level)
	var enc encoder
	if cfg.Format == "text" {
		enc = &textEncoder{}
	} else {
		enc = &jsonEncoder{}
	}
	return &logger{
		level:  level,
		output: cfg.Output,
		enc:    enc,
		fields: nil,
	}
}

// Nop returns a no-op logger that discards all output.
func Nop() Logger {
	return &nopLogger{}
}

// FromContext extracts the Logger from the context.
// Returns a no-op logger if none is set.
func FromContext(ctx context.Context) Logger {
	l, ok := ctx.Value(loggerContextKey).(Logger)
	if !ok {
		return Nop()
	}
	return l
}

// WithContext returns a new context with the given Logger.
func WithContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// exitFunc is the function called on Fatal. Replaceable for testing.
var exitFunc = os.Exit

type logger struct {
	level  *AtomicLevel
	output io.Writer
	enc    encoder
	fields []Field
}

func (l *logger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields)
}

func (l *logger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields)
}

func (l *logger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields)
}

func (l *logger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields)
}

func (l *logger) Fatal(msg string, fields ...Field) {
	l.log(LevelFatal, msg, fields)
	exitFunc(1)
}

func (l *logger) With(fields ...Field) Logger {
	combined := make([]Field, 0, len(l.fields)+len(fields))
	combined = append(combined, l.fields...)
	combined = append(combined, fields...)
	return &logger{
		level:  l.level,
		output: l.output,
		enc:    l.enc,
		fields: combined,
	}
}

func (l *logger) Level() *AtomicLevel {
	return l.level
}

func (l *logger) log(lvl Level, msg string, fields []Field) {
	if !l.level.Enabled(lvl) {
		return
	}
	allFields := l.fields
	if len(fields) > 0 {
		allFields = make([]Field, 0, len(l.fields)+len(fields))
		allFields = append(allFields, l.fields...)
		allFields = append(allFields, fields...)
	}
	data := l.enc.Encode(lvl, msg, allFields)
	data = append(data, '\n')
	_, _ = l.output.Write(data)
}

type nopLogger struct{}

func (n *nopLogger) Debug(_ string, _ ...Field) {}
func (n *nopLogger) Info(_ string, _ ...Field)  {}
func (n *nopLogger) Warn(_ string, _ ...Field)  {}
func (n *nopLogger) Error(_ string, _ ...Field) {}
func (n *nopLogger) Fatal(_ string, _ ...Field) { exitFunc(1) }
func (n *nopLogger) With(_ ...Field) Logger     { return n }
func (n *nopLogger) Level() *AtomicLevel        { return NewAtomicLevel(LevelFatal) }
