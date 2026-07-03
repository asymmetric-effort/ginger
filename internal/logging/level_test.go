package logging

import "testing"

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
		{Level(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
		ok    bool
	}{
		{"DEBUG", LevelDebug, true},
		{"debug", LevelDebug, true},
		{"INFO", LevelInfo, true},
		{"WARN", LevelWarn, true},
		{"WARNING", LevelWarn, true},
		{"ERROR", LevelError, true},
		{"FATAL", LevelFatal, true},
		{" info ", LevelInfo, true},
		{"invalid", LevelInfo, false},
		{"", LevelInfo, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseLevel(tt.input)
			if ok != tt.ok {
				t.Errorf("ParseLevel(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAtomicLevel(t *testing.T) {
	al := NewAtomicLevel(LevelInfo)
	if al.Level() != LevelInfo {
		t.Errorf("Level() = %v, want INFO", al.Level())
	}

	if !al.Enabled(LevelInfo) {
		t.Error("INFO should be enabled at INFO level")
	}
	if !al.Enabled(LevelError) {
		t.Error("ERROR should be enabled at INFO level")
	}
	if al.Enabled(LevelDebug) {
		t.Error("DEBUG should not be enabled at INFO level")
	}

	al.SetLevel(LevelDebug)
	if al.Level() != LevelDebug {
		t.Errorf("after SetLevel, Level() = %v, want DEBUG", al.Level())
	}
	if !al.Enabled(LevelDebug) {
		t.Error("DEBUG should be enabled at DEBUG level")
	}
}

func TestAtomicLevelFatal(t *testing.T) {
	al := NewAtomicLevel(LevelFatal)
	if al.Enabled(LevelError) {
		t.Error("ERROR should not be enabled at FATAL level")
	}
	if !al.Enabled(LevelFatal) {
		t.Error("FATAL should be enabled at FATAL level")
	}
}
