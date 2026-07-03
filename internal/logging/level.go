package logging

import (
	"strings"
	"sync/atomic"
)

// Level represents the severity of a log message.
type Level int32

const (
	// LevelDebug is for verbose development messages.
	LevelDebug Level = -4
	// LevelInfo is for general operational messages.
	LevelInfo Level = 0
	// LevelWarn is for messages that indicate potential issues.
	LevelWarn Level = 4
	// LevelError is for messages that indicate failures.
	LevelError Level = 8
	// LevelFatal is for messages that indicate unrecoverable failures.
	LevelFatal Level = 12
)

// String returns the human-readable name for the level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a level string (case-insensitive).
func ParseLevel(s string) (Level, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug, true
	case "INFO":
		return LevelInfo, true
	case "WARN", "WARNING":
		return LevelWarn, true
	case "ERROR":
		return LevelError, true
	case "FATAL":
		return LevelFatal, true
	default:
		return LevelInfo, false
	}
}

// AtomicLevel allows runtime-safe level adjustment.
type AtomicLevel struct {
	val atomic.Int32
}

// NewAtomicLevel creates an AtomicLevel set to the given level.
func NewAtomicLevel(l Level) *AtomicLevel {
	al := &AtomicLevel{}
	al.val.Store(int32(l))
	return al
}

// Level returns the current level.
func (al *AtomicLevel) Level() Level {
	return Level(al.val.Load())
}

// SetLevel atomically sets the level.
func (al *AtomicLevel) SetLevel(l Level) {
	al.val.Store(int32(l))
}

// Enabled reports whether the given level is enabled.
func (al *AtomicLevel) Enabled(l Level) bool {
	return l >= Level(al.val.Load())
}
