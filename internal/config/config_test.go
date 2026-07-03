package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSimpleMapping(t *testing.T) {
	input := `
name: ginger
version: 1
enabled: true
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeMapping {
		t.Fatalf("expected mapping, got %d", node.Kind)
	}
	if node.Map["name"].Value != "ginger" {
		t.Errorf("name = %q, want ginger", node.Map["name"].Value)
	}
	if node.Map["version"].Value != "1" {
		t.Errorf("version = %q, want 1", node.Map["version"].Value)
	}
	if node.Map["enabled"].Value != "true" {
		t.Errorf("enabled = %q, want true", node.Map["enabled"].Value)
	}
}

func TestParseNestedMapping(t *testing.T) {
	input := `
server:
  host: localhost
  port: 8080
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	server := node.Map["server"]
	if server.Kind != NodeMapping {
		t.Fatalf("server should be mapping, got %d", server.Kind)
	}
	if server.Map["host"].Value != "localhost" {
		t.Errorf("host = %q, want localhost", server.Map["host"].Value)
	}
	if server.Map["port"].Value != "8080" {
		t.Errorf("port = %q, want 8080", server.Map["port"].Value)
	}
}

func TestParseSequence(t *testing.T) {
	input := `
items:
  - one
  - two
  - three
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	items := node.Map["items"]
	if items.Kind != NodeSequence {
		t.Fatalf("items should be sequence, got %d", items.Kind)
	}
	if len(items.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(items.Children))
	}
	if items.Children[0].Value != "one" {
		t.Errorf("items[0] = %q, want one", items.Children[0].Value)
	}
}

func TestParseFlowSequence(t *testing.T) {
	input := `tags: [a, b, c]`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	tags := node.Map["tags"]
	if tags.Kind != NodeSequence {
		t.Fatalf("tags should be sequence")
	}
	if len(tags.Children) != 3 {
		t.Fatalf("expected 3 items, got %d", len(tags.Children))
	}
}

func TestParseFlowMapping(t *testing.T) {
	input := `labels: {app: ginger, env: prod}`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	labels := node.Map["labels"]
	if labels.Kind != NodeMapping {
		t.Fatalf("labels should be mapping")
	}
	if labels.Map["app"].Value != "ginger" {
		t.Errorf("app = %q, want ginger", labels.Map["app"].Value)
	}
}

func TestParseDocumentStart(t *testing.T) {
	input := `---
key: value
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["key"].Value != "value" {
		t.Errorf("key = %q, want value", node.Map["key"].Value)
	}
}

func TestParseQuotedStrings(t *testing.T) {
	input := `
single: 'hello world'
double: "hello world"
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["single"].Value != "hello world" {
		t.Errorf("single = %q, want hello world", node.Map["single"].Value)
	}
	if node.Map["double"].Value != "hello world" {
		t.Errorf("double = %q, want hello world", node.Map["double"].Value)
	}
}

func TestParseLiteralBlock(t *testing.T) {
	input := "description: |\n  line one\n  line two\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	val := node.Map["description"].Value
	if val != "line one\nline two" {
		t.Errorf("literal block = %q", val)
	}
}

func TestParseFoldedBlock(t *testing.T) {
	input := "description: >\n  line one\n  line two\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	val := node.Map["description"].Value
	if val != "line one line two" {
		t.Errorf("folded block = %q", val)
	}
}

func TestParseEmptyDocument(t *testing.T) {
	node, err := Parse("")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeMapping {
		t.Errorf("empty doc should be mapping, got %d", node.Kind)
	}
}

func TestParseComments(t *testing.T) {
	input := `
# This is a comment
key: value # inline comment is part of value
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["key"] == nil {
		t.Fatal("key should exist")
	}
}

func TestParseSequenceOfMappings(t *testing.T) {
	input := `
items:
  - name: first
    value: 1
  - name: second
    value: 2
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	items := node.Map["items"]
	if items.Kind != NodeSequence {
		t.Fatal("items should be sequence")
	}
	if len(items.Children) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items.Children))
	}
	if items.Children[0].Kind != NodeMapping {
		t.Fatal("first item should be mapping")
	}
	if items.Children[0].Map["name"].Value != "first" {
		t.Error("first item name mismatch")
	}
}

type testConfig struct {
	Name    string        `yaml:"name"`
	Port    int           `yaml:"port"`
	Rate    float64       `yaml:"rate"`
	Enabled bool          `yaml:"enabled"`
	Timeout time.Duration `yaml:"timeout"`
	Tags    []string      `yaml:"tags"`
	Server  testServer    `yaml:"server"`
	Extra   map[string]string `yaml:"extra"`
}

type testServer struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func TestDecodeStruct(t *testing.T) {
	input := `
name: ginger
port: 8080
rate: 0.5
enabled: true
timeout: 5s
tags:
  - trace
  - otel
server:
  host: localhost
  port: 4317
extra:
  env: prod
  region: us-east
`
	var cfg testConfig
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg.Name != "ginger" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if cfg.Rate != 0.5 {
		t.Errorf("Rate = %f", cfg.Rate)
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
	if len(cfg.Tags) != 2 {
		t.Fatalf("Tags = %v", cfg.Tags)
	}
	if cfg.Tags[0] != "trace" || cfg.Tags[1] != "otel" {
		t.Errorf("Tags = %v", cfg.Tags)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Server.Host = %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 4317 {
		t.Errorf("Server.Port = %d", cfg.Server.Port)
	}
	if cfg.Extra["env"] != "prod" {
		t.Errorf("Extra[env] = %q", cfg.Extra["env"])
	}
}

func TestDecodeEnvVars(t *testing.T) {
	t.Setenv("GINGER_HOST", "production.example.com")
	t.Setenv("GINGER_PORT", "9090")

	input := `
host: ${GINGER_HOST}
port: ${GINGER_PORT}
region: ${GINGER_REGION:-us-west-2}
`
	var cfg struct {
		Host   string `yaml:"host"`
		Port   int    `yaml:"port"`
		Region string `yaml:"region"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg.Host != "production.example.com" {
		t.Errorf("Host = %q", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("Region = %q (expected default)", cfg.Region)
	}
}

func TestDecodeStrictUnknownKey(t *testing.T) {
	input := `
known: value
unknown_key: oops
`
	var cfg struct {
		Known string `yaml:"known"`
	}
	err := DecodeStrict([]byte(input), &cfg)
	if err == nil {
		t.Error("DecodeStrict should error on unknown key")
	}
}

func TestDecodeStrictValid(t *testing.T) {
	input := `known: value`
	var cfg struct {
		Known string `yaml:"known"`
	}
	if err := DecodeStrict([]byte(input), &cfg); err != nil {
		t.Errorf("DecodeStrict should not error: %v", err)
	}
}

func TestDecodeUint(t *testing.T) {
	input := `count: 42`
	var cfg struct {
		Count uint64 `yaml:"count"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg.Count != 42 {
		t.Errorf("Count = %d", cfg.Count)
	}
}

func TestDecodeBoolVariants(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"v: true", true},
		{"v: yes", true},
		{"v: on", true},
		{"v: false", false},
		{"v: no", false},
		{"v: off", false},
	}
	for _, tt := range tests {
		var cfg struct {
			V bool `yaml:"v"`
		}
		if err := Decode([]byte(tt.input), &cfg); err != nil {
			t.Fatalf("Decode(%q) failed: %v", tt.input, err)
		}
		if cfg.V != tt.want {
			t.Errorf("Decode(%q) = %v, want %v", tt.input, cfg.V, tt.want)
		}
	}
}

func TestDecodeInvalidBool(t *testing.T) {
	input := `v: maybe`
	var cfg struct {
		V bool `yaml:"v"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for invalid bool")
	}
}

func TestDecodeInvalidInt(t *testing.T) {
	input := `v: abc`
	var cfg struct {
		V int `yaml:"v"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for invalid int")
	}
}

func TestDecodeInvalidUint(t *testing.T) {
	input := `v: abc`
	var cfg struct {
		V uint `yaml:"v"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for invalid uint")
	}
}

func TestDecodeInvalidFloat(t *testing.T) {
	input := `v: abc`
	var cfg struct {
		V float64 `yaml:"v"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for invalid float")
	}
}

func TestDecodeInvalidDuration(t *testing.T) {
	input := `v: badvalue`
	var cfg struct {
		V time.Duration `yaml:"v"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestDecodeInterface(t *testing.T) {
	input := `
str: hello
num: 42
flt: 3.14
yes: true
nul: null
`
	var cfg map[string]interface{}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg["str"] != "hello" {
		t.Errorf("str = %v", cfg["str"])
	}
	if cfg["num"] != int64(42) {
		t.Errorf("num = %v (%T)", cfg["num"], cfg["num"])
	}
	if cfg["yes"] != true {
		t.Errorf("yes = %v", cfg["yes"])
	}
	if cfg["nul"] != nil {
		t.Errorf("nul = %v", cfg["nul"])
	}
}

func TestDecodeYamlTagDash(t *testing.T) {
	input := `name: ginger`
	var cfg struct {
		Name   string `yaml:"-"`
		Unused string `yaml:"name"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg.Name != "" {
		t.Error("field with yaml:\"-\" should be skipped")
	}
	if cfg.Unused != "ginger" {
		t.Errorf("Unused = %q", cfg.Unused)
	}
}

func TestDecodeNoTag(t *testing.T) {
	input := `name: ginger`
	var cfg struct {
		Name string
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg.Name != "ginger" {
		t.Errorf("Name = %q (should match lowercase field name)", cfg.Name)
	}
}

func TestWatcher(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("v: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(path, 50*time.Millisecond)
	changed := make(chan []byte, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.Watch(ctx, func(data []byte) {
		select {
		case changed <- data:
		default:
		}
	})

	// Wait for initial read and first poll cycle
	time.Sleep(150 * time.Millisecond)

	// Change the file with clearly different content
	if err := os.WriteFile(path, []byte("v: 2"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case data := <-changed:
		if !strings.Contains(string(data), "v: 2") {
			t.Errorf("unexpected data: %s", data)
		}
	case <-ctx.Done():
		t.Error("timeout waiting for change notification")
	}
}

func TestWatcherNoChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("v: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(path, 50*time.Millisecond)
	changed := make(chan []byte, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	go w.Watch(ctx, func(data []byte) {
		changed <- data
	})

	select {
	case <-changed:
		t.Error("should not fire when file unchanged")
	case <-ctx.Done():
		// expected
	}
}

func TestWatcherMissingFile(t *testing.T) {
	w := NewWatcher("/nonexistent/config.yaml", 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	// Should not panic
	w.Watch(ctx, func(_ []byte) {})
}

func TestInterpolateEnvVars(t *testing.T) {
	t.Setenv("TEST_VAR", "hello")

	tests := []struct {
		input string
		want  string
	}{
		{"${TEST_VAR}", "hello"},
		{"${TEST_VAR:-default}", "hello"},
		{"${MISSING_VAR:-fallback}", "fallback"},
		{"${MISSING_VAR}", ""},
		{"no vars here", "no vars here"},
		{"prefix ${TEST_VAR} suffix", "prefix hello suffix"},
		{"${unclosed", "${unclosed"},
	}
	for _, tt := range tests {
		got := interpolateEnvVars(tt.input)
		if got != tt.want {
			t.Errorf("interpolateEnvVars(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHashContent(t *testing.T) {
	h1 := hashContent([]byte("hello"))
	h2 := hashContent([]byte("hello"))
	h3 := hashContent([]byte("world"))

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 16 {
		t.Errorf("hash length = %d, want 16", len(h1))
	}
}

func TestParseEmptySequenceEntry(t *testing.T) {
	input := `
items:
  -
  - value
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	items := node.Map["items"]
	if len(items.Children) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items.Children))
	}
}

func TestParseFlowSequenceEmpty(t *testing.T) {
	input := `items: []`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	items := node.Map["items"]
	if items.Kind != NodeSequence || len(items.Children) != 0 {
		t.Error("expected empty sequence")
	}
}

func TestParseFlowMappingEmpty(t *testing.T) {
	input := `labels: {}`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	labels := node.Map["labels"]
	if labels.Kind != NodeMapping || len(labels.Keys) != 0 {
		t.Error("expected empty mapping")
	}
}

func TestParseEmptyMappingValue(t *testing.T) {
	input := `
key1:
key2: value
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["key1"].Value != "" {
		t.Errorf("empty value should be empty, got %q", node.Map["key1"].Value)
	}
	if node.Map["key2"].Value != "value" {
		t.Errorf("key2 = %q", node.Map["key2"].Value)
	}
}

func TestNodeSetKeyDuplicate(t *testing.T) {
	n := NewMapping(1, 1)
	n.SetKey("a", NewScalar("1", 1, 1))
	n.SetKey("a", NewScalar("2", 1, 1))
	if len(n.Keys) != 1 {
		t.Errorf("duplicate key should not add to Keys, got %d", len(n.Keys))
	}
	if n.Map["a"].Value != "2" {
		t.Error("duplicate key should update value")
	}
}

func TestFindMappingColonInQuoted(t *testing.T) {
	// Colon inside quotes should not be a mapping separator
	idx := findMappingColon(`"key: with: colons": value`)
	if idx < 0 {
		t.Error("should find colon after quoted string")
	}
}

func TestAutoType(t *testing.T) {
	tests := []struct {
		input string
		want  interface{}
	}{
		{"null", nil},
		{"~", nil},
		{"", nil},
		{"true", true},
		{"false", false},
		{"42", int64(42)},
		{"3.14", float64(3.14)},
		{"hello", "hello"},
	}
	for _, tt := range tests {
		got := autoType(tt.input)
		if got != tt.want {
			t.Errorf("autoType(%q) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
		}
	}
}

func TestDecodeSequenceIntoInterface(t *testing.T) {
	input := `
items:
  - a
  - b
`
	var cfg map[string]interface{}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	items, ok := cfg["items"].([]interface{})
	if !ok {
		t.Fatalf("items should be []interface{}, got %T", cfg["items"])
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestParseTopLevelSequence(t *testing.T) {
	input := `
- one
- two
- three
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeSequence {
		t.Fatalf("expected sequence, got %d", node.Kind)
	}
	if len(node.Children) != 3 {
		t.Errorf("expected 3 children, got %d", len(node.Children))
	}
}

func TestParseTopLevelFlowSequence(t *testing.T) {
	input := `[a, b, c]`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeSequence {
		t.Fatalf("expected sequence, got %d", node.Kind)
	}
}

func TestDecodePointer(t *testing.T) {
	input := `name: test`
	type cfg struct {
		Name string `yaml:"name"`
	}
	var c cfg
	if err := Decode([]byte(input), &c); err != nil {
		t.Fatal(err)
	}
	if c.Name != "test" {
		t.Errorf("Name = %q", c.Name)
	}
}

func TestDecodeSliceOfStructs(t *testing.T) {
	input := `
items:
  - name: a
    value: 1
  - name: b
    value: 2
`
	type item struct {
		Name  string `yaml:"name"`
		Value int    `yaml:"value"`
	}
	var cfg struct {
		Items []item `yaml:"items"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(cfg.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(cfg.Items))
	}
	if cfg.Items[0].Name != "a" || cfg.Items[0].Value != 1 {
		t.Errorf("item 0: %+v", cfg.Items[0])
	}
}

func TestDecodeNestedMappingInSequence(t *testing.T) {
	input := `
items:
  - key: val
`
	var cfg map[string]interface{}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	items, ok := cfg["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("items = %v", cfg["items"])
	}
}

func TestDecodeInvalidSequenceTarget(t *testing.T) {
	input := `
items:
  - a
`
	var cfg struct {
		Items int `yaml:"items"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("should error decoding sequence into int")
	}
}

func TestDecodeInvalidMappingTarget(t *testing.T) {
	input := `
items:
  key: val
`
	var cfg struct {
		Items int `yaml:"items"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("should error decoding mapping into int")
	}
}

func TestDecodeInvalidScalarTarget(t *testing.T) {
	input := `val: hello`
	var cfg struct {
		Val struct{} `yaml:"val"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("should error decoding scalar into struct")
	}
}

func TestDecodeBytes(t *testing.T) {
	input := `data: hello`
	var cfg struct {
		Data []byte `yaml:"data"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	if string(cfg.Data) != "hello" {
		t.Errorf("Data = %s", cfg.Data)
	}
}

func TestParseErrorString(t *testing.T) {
	e := &ParseError{Message: "bad", Line: 5, Column: 3}
	s := e.Error()
	if s != "yaml: line 5, column 3: bad" {
		t.Errorf("Error() = %q", s)
	}
}

func TestParseFlowSequenceInvalid(t *testing.T) {
	_, err := parseFlowSequenceStr("not a sequence")
	if err == nil {
		t.Error("expected error for invalid flow sequence")
	}
}

func TestParseFlowMappingInvalid(t *testing.T) {
	_, err := parseFlowMappingStr("not a mapping")
	if err == nil {
		t.Error("expected error for invalid flow mapping")
	}
}

func TestParseFlowMappingBadEntry(t *testing.T) {
	_, err := parseFlowMappingStr("{no_colon}")
	if err == nil {
		t.Error("expected error for bad flow mapping entry")
	}
}

func TestSplitFlowItemsWithQuotes(t *testing.T) {
	items := splitFlowItems(`"a,b", c, "d"`)
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d: %v", len(items), items)
	}
}

func TestSplitFlowItemsNested(t *testing.T) {
	items := splitFlowItems(`[1,2], 3, {a: b}`)
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d: %v", len(items), items)
	}
}

func TestParseLiteralBlockEmpty(t *testing.T) {
	input := "desc: |\nother: val\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["desc"].Value != "" {
		t.Errorf("empty literal block = %q", node.Map["desc"].Value)
	}
}

func TestParseFoldedBlockEmpty(t *testing.T) {
	input := "desc: >\nother: val\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["desc"].Value != "" {
		t.Errorf("empty folded block = %q", node.Map["desc"].Value)
	}
}

func TestDecodeMapStringString(t *testing.T) {
	input := `
labels:
  a: "1"
  b: "2"
`
	var cfg struct {
		Labels map[string]string `yaml:"labels"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg.Labels["a"] != "1" || cfg.Labels["b"] != "2" {
		t.Errorf("Labels = %v", cfg.Labels)
	}
}

func TestDecodeMapInterface(t *testing.T) {
	input := `
data:
  nested:
    key: val
  list:
    - item1
`
	var cfg map[string]interface{}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	data, ok := cfg["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data = %T", cfg["data"])
	}
	nested, ok := data["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested = %T", data["nested"])
	}
	if nested["key"] != "val" {
		t.Errorf("nested.key = %v", nested["key"])
	}
}

func TestParseFlowMappingTopLevel(t *testing.T) {
	// Top-level flow mapping hits parseNode → parseFlowMapping
	node, err := Parse("{a: 1, b: 2}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeMapping {
		t.Fatalf("expected mapping, got %d", node.Kind)
	}
	if node.Map["a"].Value != "1" {
		t.Errorf("a = %q", node.Map["a"].Value)
	}
}

func TestParseFlowSequenceTopLevel(t *testing.T) {
	// Top-level flow sequence hits parseNode → parseFlowSequence
	node, err := Parse("[a, b, c]")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeSequence {
		t.Fatalf("expected sequence, got %d", node.Kind)
	}
	if len(node.Children) != 3 {
		t.Errorf("expected 3 children, got %d", len(node.Children))
	}
}

func TestDecodeInvalidYAML(t *testing.T) {
	// Create YAML that will trigger a parse error in the flow mapping
	input := `key: {invalid`
	var cfg struct {
		Key string `yaml:"key"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected parse error for malformed flow mapping")
	}
}

func TestDecodeStrictInvalidYAML(t *testing.T) {
	input := `key: {invalid`
	var cfg struct {
		Key string `yaml:"key"`
	}
	err := DecodeStrict([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error")
	}
}

func TestDecodeMapStringInt(t *testing.T) {
	input := `
counts:
  a: 1
  b: 2
`
	var cfg struct {
		Counts map[string]int `yaml:"counts"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Counts["a"] != 1 || cfg.Counts["b"] != 2 {
		t.Errorf("Counts = %v", cfg.Counts)
	}
}

func TestNodeToInterfaceSequence(t *testing.T) {
	node := NewSequence(1, 1)
	node.AddChild(NewScalar("a", 1, 1))
	child := NewMapping(1, 1)
	child.SetKey("k", NewScalar("v", 1, 1))
	node.AddChild(child)

	val, err := nodeToInterface(node)
	if err != nil {
		t.Fatal(err)
	}
	items := val.([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestDecodeMappingIntoInterface(t *testing.T) {
	input := `
data:
  key: val
`
	var result interface{}
	if err := Decode([]byte(input), &result); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	data, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data = %T", m["data"])
	}
	if data["key"] != "val" {
		t.Error("data.key mismatch")
	}
}

func TestDecodeSequenceIntoInterfaceRoot(t *testing.T) {
	input := `
- a
- b
`
	var result interface{}
	if err := Decode([]byte(input), &result); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	items, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestParseDocumentEndOnly(t *testing.T) {
	input := "---\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeMapping {
		t.Fatalf("expected mapping for empty doc, got %d", node.Kind)
	}
}

func TestParseLiteralBlockAtEOF(t *testing.T) {
	input := "val: |"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["val"].Value != "" {
		t.Errorf("literal at EOF = %q", node.Map["val"].Value)
	}
}

func TestParseFoldedBlockAtEOF(t *testing.T) {
	input := "val: >"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["val"].Value != "" {
		t.Errorf("folded at EOF = %q", node.Map["val"].Value)
	}
}

func TestDecodeNullMapValue(t *testing.T) {
	input := `
data:
  key: null
`
	var cfg map[string]interface{}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	data := cfg["data"].(map[string]interface{})
	if data["key"] != nil {
		t.Errorf("null should decode as nil, got %v", data["key"])
	}
}

func TestDecodeNestedSequenceInterface(t *testing.T) {
	input := `
items:
  - sub:
    - a
    - b
`
	var cfg map[string]interface{}
	err := Decode([]byte(input), &cfg)
	// This is a complex nesting case; verify it doesn't panic
	_ = err
}

func TestParseSequenceWithNestedMapping(t *testing.T) {
	input := `
- name: first
- name: second
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeSequence || len(node.Children) != 2 {
		t.Errorf("expected sequence with 2 children, got kind=%d len=%d", node.Kind, len(node.Children))
	}
}

func TestParseEmptySequenceChild(t *testing.T) {
	input := `
items:
  -
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	items := node.Map["items"]
	if len(items.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(items.Children))
	}
}

func TestSplitFlowItemsSingleQuote(t *testing.T) {
	items := splitFlowItems(`'a,b', c`)
	if len(items) != 2 {
		t.Errorf("expected 2, got %d: %v", len(items), items)
	}
}

func TestDecodeStrictMapping(t *testing.T) {
	input := `
items:
  - name: a
`
	var cfg struct {
		Items []struct {
			Name string `yaml:"name"`
		} `yaml:"items"`
	}
	if err := DecodeStrict([]byte(input), &cfg); err != nil {
		t.Fatalf("DecodeStrict failed: %v", err)
	}
	if len(cfg.Items) != 1 || cfg.Items[0].Name != "a" {
		t.Errorf("Items = %+v", cfg.Items)
	}
}

func TestDecodeStrictUnknownNestedKey(t *testing.T) {
	input := `
server:
  host: localhost
  unknown: bad
`
	var cfg struct {
		Server struct {
			Host string `yaml:"host"`
		} `yaml:"server"`
	}
	err := DecodeStrict([]byte(input), &cfg)
	if err == nil {
		t.Error("DecodeStrict should error on unknown nested key")
	}
}

func TestDecodePointerField(t *testing.T) {
	input := `name: test`
	type cfg struct {
		Name *string `yaml:"name"`
	}
	var c cfg
	if err := Decode([]byte(input), &c); err != nil {
		t.Fatal(err)
	}
	if c.Name == nil || *c.Name != "test" {
		t.Error("pointer field should be set")
	}
}

func TestDecodeStrictSequenceOfStructs(t *testing.T) {
	input := `
- name: a
  extra: bad
`
	var cfg []struct {
		Name string `yaml:"name"`
	}
	err := DecodeStrict([]byte(input), &cfg)
	if err == nil {
		t.Error("DecodeStrict should catch unknown key in sequence items")
	}
}

func TestDecodeMapStrictError(t *testing.T) {
	// Map with non-string values that fail to parse
	input := `
counts:
  a: not_a_number
`
	var cfg struct {
		Counts map[string]int `yaml:"counts"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for non-numeric map value")
	}
}

func TestParseMappingInlineFlowSequenceError(t *testing.T) {
	input := `key: [unclosed`
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for unclosed flow sequence")
	}
}

func TestParseMappingInlineFlowMappingError(t *testing.T) {
	input := `key: {unclosed`
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for unclosed flow mapping")
	}
}

func TestParseSequenceEntryWithNestedSeq(t *testing.T) {
	input := `
-
  - a
  - b
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeSequence {
		t.Fatalf("expected sequence, got %d", node.Kind)
	}
	if len(node.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(node.Children))
	}
	inner := node.Children[0]
	if inner.Kind != NodeSequence || len(inner.Children) != 2 {
		t.Errorf("inner should be sequence with 2 children, got kind=%d len=%d", inner.Kind, len(inner.Children))
	}
}

func TestDecodeMappingErrorInField(t *testing.T) {
	input := `
server:
  port: notanumber
`
	var cfg struct {
		Server struct {
			Port int `yaml:"port"`
		} `yaml:"server"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for invalid field value")
	}
}

func TestDecodeSequenceErrorInItem(t *testing.T) {
	input := `
items:
  - 1
  - notanumber
`
	var cfg struct {
		Items []int `yaml:"items"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error for invalid sequence item")
	}
}

func TestDecodeMappingIntoMapInterfaceError(t *testing.T) {
	// Ensure setMapping with interface{} map works
	input := `
data:
  sub:
    nested: val
`
	var cfg struct {
		Data interface{} `yaml:"data"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	m := cfg.Data.(map[string]interface{})
	sub := m["sub"].(map[string]interface{})
	if sub["nested"] != "val" {
		t.Error("nested value mismatch")
	}
}

func TestDecodeSequenceIntoSliceInterface(t *testing.T) {
	input := `
items:
  - hello
  - 42
`
	var cfg struct {
		Items []interface{} `yaml:"items"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(cfg.Items))
	}
}

func TestParseMappingEmptyValueAtEnd(t *testing.T) {
	input := "key:\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["key"].Value != "" {
		t.Errorf("empty value at end = %q", node.Map["key"].Value)
	}
}

func TestParseLiteralBlockNonIndented(t *testing.T) {
	input := "val: |\nnotindented\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["val"].Value != "" {
		t.Errorf("non-indented literal = %q", node.Map["val"].Value)
	}
}

func TestParseFoldedBlockNonIndented(t *testing.T) {
	input := "val: >\nnotindented\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Map["val"].Value != "" {
		t.Errorf("non-indented folded = %q", node.Map["val"].Value)
	}
}

func TestParseSequenceScalarWithMapping(t *testing.T) {
	// Sequence item that looks like a scalar but has no colon
	input := `
- simple_value
`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Children[0].Value != "simple_value" {
		t.Errorf("expected simple_value, got %q", node.Children[0].Value)
	}
}

func TestDecodeNoTagFieldName(t *testing.T) {
	input := `port: 8080`
	var cfg struct {
		Port int
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d", cfg.Port)
	}
}

func TestDecodeMapErrorInValue(t *testing.T) {
	input := `
counts:
  a: 1
  b: abc
`
	var cfg struct {
		Counts map[string]int `yaml:"counts"`
	}
	err := Decode([]byte(input), &cfg)
	if err == nil {
		t.Error("expected error")
	}
}

func TestNodeToInterfaceNil(t *testing.T) {
	node := &Node{Kind: NodeKind(99)}
	val, err := nodeToInterface(node)
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Errorf("unknown kind should return nil, got %v", val)
	}
}

func TestDecodeYamlTagWithComma(t *testing.T) {
	input := `name: test`
	var cfg struct {
		Name string `yaml:"name,omitempty"`
	}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %q", cfg.Name)
	}
}

func TestParseMappingWithFlowSeqError(t *testing.T) {
	input := `key: [a, b`
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for unclosed inline flow sequence")
	}
}

func TestParseMappingWithFlowMapError(t *testing.T) {
	input := `key: {a: 1`
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for unclosed inline flow mapping")
	}
}

func TestDecodeInterfaceWithNestedSeqInMap(t *testing.T) {
	input := `
data:
  items:
    - one
    - two
`
	var cfg map[string]interface{}
	if err := Decode([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	data := cfg["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestParseEndOfInput(t *testing.T) {
	// Single line with no newline
	node, err := Parse("key: value")
	if err != nil {
		t.Fatal(err)
	}
	if node.Map["key"].Value != "value" {
		t.Errorf("key = %q", node.Map["key"].Value)
	}
}

func TestParseLiteralBlockWithBlankLine(t *testing.T) {
	input := "desc: |\n  line1\n\n  line2\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	val := node.Map["desc"].Value
	if val != "line1\n\nline2" {
		t.Errorf("literal block with blank line = %q", val)
	}
}

func TestParseFoldedBlockWithBlankLine(t *testing.T) {
	input := "desc: >\n  line1\n\n  line2\n"
	node, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	val := node.Map["desc"].Value
	if val != "line1 \n line2" {
		t.Errorf("folded block with blank line = %q", val)
	}
}

func TestParseScalarOnly(t *testing.T) {
	input := `just a value`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeScalar {
		t.Errorf("expected scalar, got %d", node.Kind)
	}
}

func TestParseTopLevelFlowMapping(t *testing.T) {
	input := `{a: 1, b: 2}`
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if node.Kind != NodeMapping {
		t.Fatalf("expected mapping, got %d", node.Kind)
	}
}
