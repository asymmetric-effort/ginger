package config

import (
	"fmt"
	"strings"
)

// ParseError represents a YAML parse error with position information.
type ParseError struct {
	Message string
	Line    int
	Column  int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("yaml: line %d, column %d: %s", e.Line, e.Column, e.Message)
}

// Parse parses YAML input into an AST node.
func Parse(input string) (*Node, error) {
	lines := splitLines(input)
	p := &parser{lines: lines}
	return p.parseDocument()
}

type parser struct {
	lines []parseLine
	pos   int
}

type parseLine struct {
	content string
	indent  int
	lineNum int
	isEmpty bool
}

func splitLines(input string) []parseLine {
	raw := strings.Split(input, "\n")
	lines := make([]parseLine, 0, len(raw))
	for i, line := range raw {
		trimmed := strings.TrimRight(line, " \t\r")
		content := strings.TrimLeft(trimmed, " ")
		indent := len(trimmed) - len(content)
		isEmpty := content == "" || strings.HasPrefix(content, "#")
		lines = append(lines, parseLine{
			content: content,
			indent:  indent,
			lineNum: i + 1,
			isEmpty: isEmpty,
		})
	}
	return lines
}

func (p *parser) parseDocument() (*Node, error) {
	p.skipEmpty()

	// Skip document start marker
	if p.pos < len(p.lines) && p.lines[p.pos].content == "---" {
		p.pos++
		p.skipEmpty()
	}

	if p.pos >= len(p.lines) {
		return NewMapping(1, 1), nil
	}

	node, err := p.parseNode(-1)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (p *parser) parseNode(parentIndent int) (*Node, error) {
	p.skipEmpty()
	if p.pos >= len(p.lines) {
		return NewScalar("", 0, 0), nil
	}

	line := p.lines[p.pos]

	// Check for sequence
	if strings.HasPrefix(line.content, "- ") || line.content == "-" {
		return p.parseSequence(line.indent)
	}

	// Flow sequence (check before mapping since flow items may contain colons)
	if strings.HasPrefix(line.content, "[") {
		return p.parseFlowSequence()
	}

	// Flow mapping (check before mapping since flow items may contain colons)
	if strings.HasPrefix(line.content, "{") {
		return p.parseFlowMapping()
	}

	// Check for mapping
	if idx := findMappingColon(line.content); idx > 0 {
		return p.parseMapping(line.indent)
	}

	// Scalar
	p.pos++
	return NewScalar(unquoteScalar(line.content), line.lineNum, line.indent+1), nil
}

func (p *parser) parseMapping(baseIndent int) (*Node, error) {
	node := NewMapping(p.lines[p.pos].lineNum, baseIndent+1)

	for p.pos < len(p.lines) {
		p.skipEmpty()
		if p.pos >= len(p.lines) {
			break
		}
		line := p.lines[p.pos]
		if line.indent < baseIndent {
			break
		}
		if line.indent > baseIndent && node.Map != nil && len(node.Keys) > 0 {
			break
		}
		if line.indent != baseIndent {
			break
		}

		idx := findMappingColon(line.content)
		if idx <= 0 {
			break
		}

		key := strings.TrimSpace(line.content[:idx])
		valueStr := strings.TrimSpace(line.content[idx+1:])

		if valueStr == "" || valueStr == "|" || valueStr == ">" {
			// Value on next line(s) or multi-line string
			if valueStr == "|" {
				p.pos++
				val, err := p.parseLiteralBlock(baseIndent)
				if err != nil {
					return nil, err
				}
				node.SetKey(key, NewScalar(val, line.lineNum, line.indent+1))
				continue
			}
			if valueStr == ">" {
				p.pos++
				val, err := p.parseFoldedBlock(baseIndent)
				if err != nil {
					return nil, err
				}
				node.SetKey(key, NewScalar(val, line.lineNum, line.indent+1))
				continue
			}
			p.pos++
			p.skipEmpty()
			if p.pos >= len(p.lines) {
				node.SetKey(key, NewScalar("", line.lineNum, line.indent+1))
				continue
			}
			childLine := p.lines[p.pos]
			if childLine.indent <= baseIndent {
				node.SetKey(key, NewScalar("", line.lineNum, line.indent+1))
				continue
			}
			child, err := p.parseNode(baseIndent)
			if err != nil {
				return nil, err
			}
			node.SetKey(key, child)
		} else {
			p.pos++
			// Inline value — could be a flow sequence/mapping
			if strings.HasPrefix(valueStr, "[") {
				child, err := parseFlowSequenceStr(valueStr)
				if err != nil {
					return nil, &ParseError{Message: err.Error(), Line: line.lineNum, Column: idx + 2}
				}
				node.SetKey(key, child)
			} else if strings.HasPrefix(valueStr, "{") {
				child, err := parseFlowMappingStr(valueStr)
				if err != nil {
					return nil, &ParseError{Message: err.Error(), Line: line.lineNum, Column: idx + 2}
				}
				node.SetKey(key, child)
			} else {
				node.SetKey(key, NewScalar(unquoteScalar(valueStr), line.lineNum, idx+2))
			}
		}
	}

	return node, nil
}

func (p *parser) parseSequence(baseIndent int) (*Node, error) {
	node := NewSequence(p.lines[p.pos].lineNum, baseIndent+1)

	for p.pos < len(p.lines) {
		p.skipEmpty()
		if p.pos >= len(p.lines) {
			break
		}
		line := p.lines[p.pos]
		if line.indent != baseIndent {
			break
		}
		if !strings.HasPrefix(line.content, "- ") && line.content != "-" {
			break
		}

		var valueStr string
		if line.content == "-" {
			valueStr = ""
		} else {
			valueStr = strings.TrimSpace(line.content[2:])
		}

		if valueStr == "" {
			p.pos++
			p.skipEmpty()
			if p.pos < len(p.lines) && p.lines[p.pos].indent > baseIndent {
				child, err := p.parseNode(baseIndent)
				if err != nil {
					return nil, err
				}
				node.AddChild(child)
			} else {
				node.AddChild(NewScalar("", line.lineNum, baseIndent+3))
			}
		} else if findMappingColon(valueStr) > 0 {
			// Inline mapping in sequence entry
			child := NewMapping(line.lineNum, baseIndent+3)
			idx := findMappingColon(valueStr)
			k := strings.TrimSpace(valueStr[:idx])
			v := strings.TrimSpace(valueStr[idx+1:])
			child.SetKey(k, NewScalar(unquoteScalar(v), line.lineNum, baseIndent+3))
			p.pos++
			// Check for continuation keys at indent > baseIndent
			for p.pos < len(p.lines) {
				p.skipEmpty()
				if p.pos >= len(p.lines) {
					break
				}
				next := p.lines[p.pos]
				if next.indent <= baseIndent {
					break
				}
				cidx := findMappingColon(next.content)
				if cidx <= 0 {
					break
				}
				ck := strings.TrimSpace(next.content[:cidx])
				cv := strings.TrimSpace(next.content[cidx+1:])
				child.SetKey(ck, NewScalar(unquoteScalar(cv), next.lineNum, next.indent+1))
				p.pos++
			}
			node.AddChild(child)
		} else {
			p.pos++
			node.AddChild(NewScalar(unquoteScalar(valueStr), line.lineNum, baseIndent+3))
		}
	}

	return node, nil
}

func (p *parser) parseLiteralBlock(parentIndent int) (string, error) {
	p.skipEmpty()
	if p.pos >= len(p.lines) {
		return "", nil
	}
	blockIndent := p.lines[p.pos].indent
	if blockIndent <= parentIndent {
		return "", nil
	}

	var lines []string
	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		if !line.isEmpty && line.indent < blockIndent {
			break
		}
		if line.isEmpty {
			lines = append(lines, "")
		} else {
			lines = append(lines, line.content)
		}
		p.pos++
	}
	// Trim trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n"), nil
}

func (p *parser) parseFoldedBlock(parentIndent int) (string, error) {
	p.skipEmpty()
	if p.pos >= len(p.lines) {
		return "", nil
	}
	blockIndent := p.lines[p.pos].indent
	if blockIndent <= parentIndent {
		return "", nil
	}

	var parts []string
	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		if !line.isEmpty && line.indent < blockIndent {
			break
		}
		if line.isEmpty {
			parts = append(parts, "\n")
		} else {
			parts = append(parts, line.content)
		}
		p.pos++
	}
	// Trim trailing empty parts
	for len(parts) > 0 && (parts[len(parts)-1] == "" || parts[len(parts)-1] == "\n") {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, " "), nil
}

func (p *parser) parseFlowSequence() (*Node, error) {
	line := p.lines[p.pos]
	p.pos++
	return parseFlowSequenceStr(line.content)
}

func (p *parser) parseFlowMapping() (*Node, error) {
	line := p.lines[p.pos]
	p.pos++
	return parseFlowMappingStr(line.content)
}

func parseFlowSequenceStr(s string) (*Node, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("invalid flow sequence")
	}
	inner := s[1 : len(s)-1]
	node := NewSequence(0, 0)
	if strings.TrimSpace(inner) == "" {
		return node, nil
	}
	parts := splitFlowItems(inner)
	for _, part := range parts {
		node.AddChild(NewScalar(unquoteScalar(strings.TrimSpace(part)), 0, 0))
	}
	return node, nil
}

func parseFlowMappingStr(s string) (*Node, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("invalid flow mapping")
	}
	inner := s[1 : len(s)-1]
	node := NewMapping(0, 0)
	if strings.TrimSpace(inner) == "" {
		return node, nil
	}
	parts := splitFlowItems(inner)
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid flow mapping entry: %q", part)
		}
		node.SetKey(strings.TrimSpace(kv[0]), NewScalar(unquoteScalar(strings.TrimSpace(kv[1])), 0, 0))
	}
	return node, nil
}

func splitFlowItems(s string) []string {
	var items []string
	depth := 0
	start := 0
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == '[' || c == '{' {
			depth++
		}
		if c == ']' || c == '}' {
			depth--
		}
		if c == ',' && depth == 0 {
			items = append(items, s[start:i])
			start = i + 1
		}
	}
	items = append(items, s[start:])
	return items
}

func (p *parser) skipEmpty() {
	for p.pos < len(p.lines) && p.lines[p.pos].isEmpty {
		p.pos++
	}
}

// findMappingColon finds the colon that separates a mapping key from its value.
// Returns -1 if not found. Handles quoted strings.
func findMappingColon(s string) int {
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == ':' && (i+1 >= len(s) || s[i+1] == ' ' || s[i+1] == '\t') {
			return i
		}
	}
	return -1
}

func unquoteScalar(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
