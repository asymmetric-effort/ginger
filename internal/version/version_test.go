package version

import (
	"fmt"
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	v := String()
	if !strings.HasPrefix(v, "ginger v") {
		t.Errorf("expected version to start with 'ginger v', got %q", v)
	}
}

func TestStringContainsVersion(t *testing.T) {
	v := String()
	expected := fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)
	if !strings.Contains(v, expected) {
		t.Errorf("expected version to contain %q, got %q", expected, v)
	}
}

func TestStringContainsPrerelease(t *testing.T) {
	v := String()
	if Pre != "" && !strings.HasSuffix(v, Pre) {
		t.Errorf("expected version to end with %q, got %q", Pre, v)
	}
}
