package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidationError describes a config validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateFile reads and validates a YAML config file. Returns all errors found.
func ValidateFile(path string, strict bool) ([]ValidationError, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return ValidateBytes(data, strict)
}

// ValidateBytes validates YAML config bytes.
func ValidateBytes(data []byte, strict bool) ([]ValidationError, error) {
	expanded := interpolateEnvVars(string(data))
	node, err := Parse(expanded)
	if err != nil {
		return nil, err
	}
	var errs []ValidationError
	validateNode("", node, &errs, strict)
	return errs, nil
}

func validateNode(prefix string, node *Node, errs *[]ValidationError, strict bool) {
	switch node.Kind {
	case NodeMapping:
		for _, key := range node.Keys {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			child := node.Map[key]
			validateNode(path, child, errs, strict)
		}
	case NodeSequence:
		for i, child := range node.Children {
			path := fmt.Sprintf("%s[%d]", prefix, i)
			validateNode(path, child, errs, strict)
		}
	case NodeScalar:
		// Validate known patterns
		if strings.HasSuffix(prefix, ".port") || strings.HasSuffix(prefix, "_port") {
			validatePort(prefix, node.Value, errs)
		}
	}
}

func validatePort(field, value string, errs *[]ValidationError) {
	if value == "" {
		return
	}
	port := 0
	for _, c := range value {
		if c < '0' || c > '9' {
			*errs = append(*errs, ValidationError{Field: field, Message: fmt.Sprintf("invalid port: %q", value)})
			return
		}
		port = port*10 + int(c-'0')
	}
	if port < 1 || port > 65535 {
		*errs = append(*errs, ValidationError{Field: field, Message: fmt.Sprintf("port %d out of range 1-65535", port)})
	}
}
