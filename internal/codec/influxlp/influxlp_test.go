package influxlp

import (
	"strings"
	"testing"
	"time"
)

func TestEncodeBasic(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "cpu",
		Tags:        []Tag{{Key: "host", Value: "server01"}},
		Fields:      []Field{FloatField("value", 0.64)},
		Timestamp:   time.Unix(0, 1435362189575692182),
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if !strings.HasPrefix(line, "cpu,host=server01 value=0.64 ") {
		t.Errorf("unexpected: %s", line)
	}
}

func TestEncodeMultipleTags(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "mem",
		Tags:        []Tag{{Key: "region", Value: "us"}, {Key: "host", Value: "a"}},
		Fields:      []Field{IntField("used", 1024)},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	// Tags should be sorted: host before region
	if !strings.Contains(line, "mem,host=a,region=us") {
		t.Errorf("tags not sorted: %s", line)
	}
	if !strings.Contains(line, "used=1024i") {
		t.Errorf("missing int field: %s", line)
	}
}

func TestEncodeAllFieldTypes(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "test",
		Fields: []Field{
			FloatField("f", 3.14),
			IntField("i", 42),
			UIntField("u", 100),
			BoolField("b", true),
			StringField("s", "hello"),
		},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if !strings.Contains(line, "f=3.14") {
		t.Errorf("float: %s", line)
	}
	if !strings.Contains(line, "i=42i") {
		t.Errorf("int: %s", line)
	}
	if !strings.Contains(line, "u=100u") {
		t.Errorf("uint: %s", line)
	}
	if !strings.Contains(line, "b=t") {
		t.Errorf("bool: %s", line)
	}
	if !strings.Contains(line, `s="hello"`) {
		t.Errorf("string: %s", line)
	}
}

func TestEncodeBoolFalse(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "test",
		Fields:      []Field{BoolField("active", false)},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "active=f") {
		t.Errorf("false bool: %s", data)
	}
}

func TestEncodeNoTimestamp(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "cpu",
		Fields:      []Field{FloatField("v", 1.0)},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if strings.Contains(line, " 0") || strings.Count(line, " ") > 1 {
		t.Errorf("no timestamp should have no trailing space+number: %s", line)
	}
}

func TestEncodeNoMeasurement(t *testing.T) {
	enc := NewEncoder()
	p := Point{Fields: []Field{FloatField("v", 1.0)}}
	_, err := enc.Encode(p)
	if err == nil {
		t.Error("expected error for empty measurement")
	}
}

func TestEncodeNoFields(t *testing.T) {
	enc := NewEncoder()
	p := Point{Measurement: "cpu"}
	_, err := enc.Encode(p)
	if err == nil {
		t.Error("expected error for no fields")
	}
}

func TestEncodeEscaping(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "my measurement",
		Tags:        []Tag{{Key: "tag,key", Value: "tag=value"}},
		Fields:      []Field{StringField("field key", `say "hello"`)},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if !strings.Contains(line, `my\ measurement`) {
		t.Errorf("measurement not escaped: %s", line)
	}
	if !strings.Contains(line, `tag\,key=tag\=value`) {
		t.Errorf("tag not escaped: %s", line)
	}
	if !strings.Contains(line, `field\ key="say \"hello\""`) {
		t.Errorf("field not escaped: %s", line)
	}
}

func TestEncodeMeasurementComma(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "cpu,usage",
		Fields:      []Field{FloatField("v", 1.0)},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), `cpu\,usage`) {
		t.Errorf("comma not escaped: %s", data)
	}
}

func TestEncodeFieldStringBackslash(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "test",
		Fields:      []Field{StringField("path", `c:\temp`)},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"c:\\temp"`) {
		t.Errorf("backslash not escaped: %s", data)
	}
}

func TestEncodeMulti(t *testing.T) {
	enc := NewEncoder()
	points := []Point{
		{Measurement: "a", Fields: []Field{IntField("v", 1)}},
		{Measurement: "b", Fields: []Field{IntField("v", 2)}},
	}
	data, err := enc.EncodeMulti(points)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestEncodeMultiError(t *testing.T) {
	enc := NewEncoder()
	points := []Point{
		{Measurement: "", Fields: []Field{IntField("v", 1)}},
	}
	_, err := enc.EncodeMulti(points)
	if err == nil {
		t.Error("expected error")
	}
}

func TestEncodePrecisions(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 123456789, time.UTC)
	p := Point{Measurement: "m", Fields: []Field{IntField("v", 1)}, Timestamp: ts}

	tests := []struct {
		prec Precision
	}{
		{PrecisionNanosecond},
		{PrecisionMicrosecond},
		{PrecisionMillisecond},
		{PrecisionSecond},
	}
	for _, tt := range tests {
		enc := &Encoder{Precision: tt.prec}
		data, err := enc.Encode(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) == 0 {
			t.Error("expected non-empty output")
		}
	}
}

// === Decoder Tests ===

func TestDecodeBasic(t *testing.T) {
	dec := NewDecoder()
	points, err := dec.Decode([]byte("cpu,host=server01 value=0.64 1435362189575692182"))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	p := points[0]
	if p.Measurement != "cpu" {
		t.Errorf("measurement = %q", p.Measurement)
	}
	if len(p.Tags) != 1 || p.Tags[0].Key != "host" || p.Tags[0].Value != "server01" {
		t.Errorf("tags = %v", p.Tags)
	}
	if len(p.Fields) != 1 || p.Fields[0].Key != "value" || p.Fields[0].Value.Float != 0.64 {
		t.Errorf("fields = %v", p.Fields)
	}
	if p.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestDecodeNoTags(t *testing.T) {
	dec := NewDecoder()
	points, err := dec.Decode([]byte("cpu value=0.5"))
	if err != nil {
		t.Fatal(err)
	}
	if points[0].Measurement != "cpu" {
		t.Errorf("measurement = %q", points[0].Measurement)
	}
	if len(points[0].Tags) != 0 {
		t.Error("expected no tags")
	}
}

func TestDecodeAllFieldTypes(t *testing.T) {
	dec := NewDecoder()
	points, err := dec.Decode([]byte(`test f=3.14,i=42i,u=100u,b=t,s="hello"`))
	if err != nil {
		t.Fatal(err)
	}
	p := points[0]
	fieldMap := make(map[string]FieldValue)
	for _, f := range p.Fields {
		fieldMap[f.Key] = f.Value
	}

	if fieldMap["f"].Type != FieldTypeFloat || fieldMap["f"].Float != 3.14 {
		t.Error("float field")
	}
	if fieldMap["i"].Type != FieldTypeInt || fieldMap["i"].Int != 42 {
		t.Error("int field")
	}
	if fieldMap["u"].Type != FieldTypeUInt || fieldMap["u"].UInt != 100 {
		t.Error("uint field")
	}
	if fieldMap["b"].Type != FieldTypeBool || !fieldMap["b"].Bool {
		t.Error("bool field")
	}
	if fieldMap["s"].Type != FieldTypeString || fieldMap["s"].String != "hello" {
		t.Error("string field")
	}
}

func TestDecodeBoolVariants(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`m b=t`, true}, {`m b=T`, true}, {`m b=true`, true}, {`m b=True`, true}, {`m b=TRUE`, true},
		{`m b=f`, false}, {`m b=F`, false}, {`m b=false`, false}, {`m b=False`, false}, {`m b=FALSE`, false},
	}
	dec := NewDecoder()
	for _, tt := range tests {
		points, err := dec.Decode([]byte(tt.input))
		if err != nil {
			t.Fatalf("decode %q: %v", tt.input, err)
		}
		if points[0].Fields[0].Value.Bool != tt.want {
			t.Errorf("%q: got %v, want %v", tt.input, points[0].Fields[0].Value.Bool, tt.want)
		}
	}
}

func TestDecodeEscaping(t *testing.T) {
	dec := NewDecoder()
	points, err := dec.Decode([]byte(`my\ measurement,tag\,key=tag\=value field\ key="say \"hello\""`))
	if err != nil {
		t.Fatal(err)
	}
	p := points[0]
	if p.Measurement != "my measurement" {
		t.Errorf("measurement = %q", p.Measurement)
	}
	if p.Tags[0].Key != "tag,key" || p.Tags[0].Value != "tag=value" {
		t.Errorf("tags = %v", p.Tags)
	}
	if p.Fields[0].Key != "field key" || p.Fields[0].Value.String != `say "hello"` {
		t.Errorf("field = %v", p.Fields[0])
	}
}

func TestDecodeMultiLine(t *testing.T) {
	dec := NewDecoder()
	input := "a v=1i\nb v=2i\n\n# comment\nc v=3i"
	points, err := dec.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 {
		t.Errorf("expected 3 points, got %d", len(points))
	}
}

func TestDecodeMissingFields(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu"))
	if err == nil {
		t.Error("expected error for missing fields")
	}
}

func TestDecodeInvalidTag(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu,badtag value=1"))
	if err == nil {
		t.Error("expected error for invalid tag")
	}
}

func TestDecodeInvalidField(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu badfield"))
	if err == nil {
		t.Error("expected error for invalid field")
	}
}

func TestDecodeInvalidTimestamp(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu v=1i notanumber"))
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestDecodeEmptyFieldValue(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu v="))
	if err == nil {
		t.Error("expected error for empty field value")
	}
}

func TestDecodeUnterminatedString(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte(`cpu v="unterminated`))
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}

func TestDecodeInvalidInt(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu v=abci"))
	if err == nil {
		t.Error("expected error for invalid int")
	}
}

func TestDecodeInvalidUint(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu v=abcu"))
	if err == nil {
		t.Error("expected error for invalid uint")
	}
}

func TestDecodeInvalidFloat(t *testing.T) {
	dec := NewDecoder()
	_, err := dec.Decode([]byte("cpu v=abc"))
	if err == nil {
		t.Error("expected error for invalid float")
	}
}

func TestDecodePrecisions(t *testing.T) {
	tests := []struct {
		prec Precision
		ts   string
	}{
		{PrecisionSecond, "1609459200"},
		{PrecisionMillisecond, "1609459200000"},
		{PrecisionMicrosecond, "1609459200000000"},
		{PrecisionNanosecond, "1609459200000000000"},
	}
	for _, tt := range tests {
		dec := &Decoder{Precision: tt.prec}
		points, err := dec.Decode([]byte("m v=1i " + tt.ts))
		if err != nil {
			t.Fatal(err)
		}
		if points[0].Timestamp.Year() != 2021 {
			t.Errorf("prec %d: year = %d", tt.prec, points[0].Timestamp.Year())
		}
	}
}

func TestRoundTrip(t *testing.T) {
	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	original := Point{
		Measurement: "traces",
		Tags:        []Tag{{Key: "service", Value: "ginger"}, {Key: "host", Value: "node01"}},
		Fields: []Field{
			IntField("duration_us", 1500),
			StringField("name", "span1"),
			BoolField("error", false),
			FloatField("rate", 0.95),
		},
		Timestamp: ts,
	}

	enc := NewEncoder()
	data, err := enc.Encode(original)
	if err != nil {
		t.Fatal(err)
	}

	dec := NewDecoder()
	points, err := dec.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v\nline: %s", err, data)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}

	p := points[0]
	if p.Measurement != "traces" {
		t.Errorf("measurement = %q", p.Measurement)
	}
	if len(p.Tags) != 2 {
		t.Errorf("tags = %v", p.Tags)
	}
	if len(p.Fields) != 4 {
		t.Errorf("fields = %v", p.Fields)
	}
}

func TestDecodeEmpty(t *testing.T) {
	dec := NewDecoder()
	points, err := dec.Decode([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 0 {
		t.Errorf("expected 0 points, got %d", len(points))
	}
}

func TestDecodeCommentsOnly(t *testing.T) {
	dec := NewDecoder()
	points, err := dec.Decode([]byte("# comment\n# another"))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 0 {
		t.Errorf("expected 0 points, got %d", len(points))
	}
}

func TestDecodeFieldStringBackslash(t *testing.T) {
	dec := NewDecoder()
	points, err := dec.Decode([]byte(`test path="c:\\temp"`))
	if err != nil {
		t.Fatal(err)
	}
	if points[0].Fields[0].Value.String != `c:\temp` {
		t.Errorf("string = %q", points[0].Fields[0].Value.String)
	}
}

func TestEncodeNoTags(t *testing.T) {
	enc := NewEncoder()
	p := Point{Measurement: "cpu", Fields: []Field{FloatField("v", 1.0)}}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), ",") {
		t.Errorf("no tags should have no commas: %s", data)
	}
}

func TestEncodeFieldsSorted(t *testing.T) {
	enc := NewEncoder()
	p := Point{
		Measurement: "m",
		Fields: []Field{
			IntField("z", 1),
			IntField("a", 2),
			IntField("m", 3),
		},
	}
	data, err := enc.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	aIdx := strings.Index(line, "a=")
	mIdx := strings.Index(line, "m=")
	zIdx := strings.Index(line, "z=")
	if aIdx > mIdx || mIdx > zIdx {
		t.Errorf("fields not sorted: %s", line)
	}
}
