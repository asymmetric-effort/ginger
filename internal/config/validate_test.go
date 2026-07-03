package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBytesValid(t *testing.T) {
	input := `
server:
  port: 8080
  host: localhost
`
	errs, err := ValidateBytes([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %v", errs)
	}
}

func TestValidateBytesInvalidPort(t *testing.T) {
	input := `
server:
  port: 99999
`
	errs, err := ValidateBytes([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Field != "server.port" {
		t.Errorf("field = %q", errs[0].Field)
	}
}

func TestValidateBytesNonNumericPort(t *testing.T) {
	input := `server_port: abc`
	errs, _ := ValidateBytes([]byte(input), false)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateBytesEmptyPort(t *testing.T) {
	input := `server_port:`
	errs, _ := ValidateBytes([]byte(input), false)
	if len(errs) != 0 {
		t.Errorf("empty port should be valid: %v", errs)
	}
}

func TestValidateBytesWithSequence(t *testing.T) {
	input := `
items:
  - name: a
  - name: b
`
	errs, err := ValidateBytes([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Errorf("valid sequence should have no errors: %v", errs)
	}
}

func TestValidateFileSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("server:\n  port: 4317\n"), 0644)

	errs, err := ValidateFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Errorf("errors: %v", errs)
	}
}

func TestValidateFileNotFound(t *testing.T) {
	_, err := ValidateFile("/nonexistent", false)
	if err == nil {
		t.Error("should error for missing file")
	}
}

func TestValidationErrorString(t *testing.T) {
	e := ValidationError{Field: "server.port", Message: "out of range"}
	s := e.String()
	if s != "server.port: out of range" {
		t.Errorf("String() = %q", s)
	}
}
