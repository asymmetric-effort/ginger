package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	l.Info("hello", String("key", "value"), Int("count", 42))

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if entry["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", entry["level"])
	}
	if entry["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("key = %v, want value", entry["key"])
	}
	if entry["count"] != float64(42) {
		t.Errorf("count = %v, want 42", entry["count"])
	}
	if _, ok := entry["ts"]; !ok {
		t.Error("missing ts field")
	}
}

func TestLoggerText(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "text"})

	l.Info("hello", String("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("text output missing INFO: %s", output)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("text output missing message: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("text output missing key=value: %s", output)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelWarn, Output: &buf, Format: "json"})

	l.Debug("should not appear")
	l.Info("should not appear")

	if buf.Len() > 0 {
		t.Errorf("debug/info should be filtered at WARN level, got: %s", buf.String())
	}

	l.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("warn should appear at WARN level")
	}
}

func TestLoggerRuntimeLevelChange(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelError, Output: &buf, Format: "json"})

	l.Info("hidden")
	if buf.Len() > 0 {
		t.Error("info should be hidden at ERROR level")
	}

	l.Level().SetLevel(LevelDebug)
	l.Info("visible")
	if buf.Len() == 0 {
		t.Error("info should be visible after level change to DEBUG")
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	child := l.With(String("service", "ginger"))
	child.Info("test")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["service"] != "ginger" {
		t.Errorf("child logger should include parent fields, got: %v", entry)
	}
}

func TestLoggerWithChain(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	child := l.With(String("a", "1")).With(String("b", "2"))
	child.Info("test")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["a"] != "1" || entry["b"] != "2" {
		t.Errorf("chained With should include all fields: %v", entry)
	}
}

func TestLoggerAllLevels(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 log lines, got %d", len(lines))
	}

	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for i, line := range lines {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("line %d invalid JSON: %v", i, err)
		}
		if entry["level"] != levels[i] {
			t.Errorf("line %d level = %v, want %s", i, entry["level"], levels[i])
		}
	}
}

func TestLoggerAllFieldTypes(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.Info("test",
		String("s", "val"),
		Int64("i", 99),
		Float64("f", 1.5),
		Bool("b", true),
		Error(errors.New("fail")),
		Duration("d", 5*time.Second),
		Time("t", now),
	)

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["s"] != "val" {
		t.Error("string field mismatch")
	}
	if entry["i"] != float64(99) {
		t.Error("int field mismatch")
	}
	if entry["f"] != 1.5 {
		t.Error("float field mismatch")
	}
	if entry["b"] != true {
		t.Error("bool field mismatch")
	}
	if entry["error"] != "fail" {
		t.Error("error field mismatch")
	}
}

func TestLoggerNilError(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	l.Info("test", NamedError("err", nil))

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["err"] != nil {
		t.Errorf("nil error should serialize as null, got %v", entry["err"])
	}
}

func TestLoggerJSONEscaping(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	l.Info("message with \"quotes\" and \nnewline", String("key", "val\twith\ttabs"))

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("JSON escaping failed: %v\noutput: %s", err, buf.String())
	}
}

func TestLoggerDefaultOutput(t *testing.T) {
	l := New(Config{Level: LevelInfo})
	// Should not panic when Output is nil (defaults to os.Stderr)
	_ = l
}

func TestLoggerDefaultFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf})
	l.Info("test")

	// Default should be JSON
	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("default format should be JSON: %v", err)
	}
}

func TestNopLogger(t *testing.T) {
	l := Nop()
	l.Debug("ignored")
	l.Info("ignored")
	l.Warn("ignored")
	l.Error("ignored")

	child := l.With(String("key", "val"))
	child.Info("also ignored")

	if l.Level().Level() != LevelFatal {
		t.Error("nop logger should have FATAL level")
	}
}

func TestFromContext(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	ctx := context.Background()
	ctx = WithContext(ctx, l)

	retrieved := FromContext(ctx)
	retrieved.Info("from context")

	if buf.Len() == 0 {
		t.Error("logger from context should produce output")
	}
}

func TestFromContextEmpty(t *testing.T) {
	l := FromContext(context.Background())
	// Should return nop logger, not panic
	l.Info("should not panic")
}

func TestTextEncoderAllFields(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "text"})

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.Info("test",
		String("s", "val"),
		Int64("i", 99),
		Float64("f", 1.5),
		Bool("b", true),
		Error(errors.New("fail")),
		NamedError("nilErr", nil),
		Duration("d", 5*time.Second),
		Time("t", now),
	)

	output := buf.String()
	if !strings.Contains(output, "s=val") {
		t.Errorf("text output missing s=val: %s", output)
	}
	if !strings.Contains(output, "i=99") {
		t.Errorf("text output missing i=99: %s", output)
	}
	if !strings.Contains(output, "b=true") {
		t.Errorf("text output missing b=true: %s", output)
	}
	if !strings.Contains(output, "error=fail") {
		t.Errorf("text output missing error=fail: %s", output)
	}
	if !strings.Contains(output, "nilErr=<nil>") {
		t.Errorf("text output missing nilErr=<nil>: %s", output)
	}
}

func TestLoggerNoFieldsOptimization(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	l.Info("no fields")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(entry) != 3 { // level, ts, msg
		t.Errorf("expected 3 fields (level, ts, msg), got %d", len(entry))
	}
}

func TestTextEncoderUnknownFieldType(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "text"})

	// Use an unknown field type
	f := Field{Key: "unknown", Type: FieldType(99)}
	l.Info("test", f)
	output := buf.String()
	if !strings.Contains(output, "unknown=<unknown>") {
		t.Errorf("text encoder should handle unknown type: %s", output)
	}
}

func TestJSONEncoderUnknownFieldType(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	f := Field{Key: "unknown", Type: FieldType(99)}
	l.Info("test", f)

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["unknown"] != nil {
		t.Errorf("unknown field type should serialize as null, got %v", entry["unknown"])
	}
}

func TestJSONEscapeBackslashAndReturn(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	l.Info("msg with \\backslash and \rreturn")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("JSON escaping failed: %v\noutput: %s", err, buf.String())
	}
}

func TestNopLoggerMethods(t *testing.T) {
	l := Nop()
	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")
	child := l.With(String("k", "v"))
	child.Debug("child")
	_ = l.Level()
}

func TestLoggerFatal(t *testing.T) {
	var exitCode int
	origExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = origExit }()

	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})
	l.Fatal("fatal message")

	if exitCode != 1 {
		t.Errorf("Fatal should call exit with code 1, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "FATAL") {
		t.Error("Fatal should log at FATAL level")
	}
}

func TestNopLoggerFatal(t *testing.T) {
	var exitCode int
	origExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = origExit }()

	l := Nop()
	l.Fatal("fatal")

	if exitCode != 1 {
		t.Errorf("nop Fatal should call exit with code 1, got %d", exitCode)
	}
}

func TestLoggerControlCharEscaping(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: LevelDebug, Output: &buf, Format: "json"})

	l.Info("msg\x00\x01\x02")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("control char escaping failed: %v\noutput: %s", err, buf.String())
	}
}
