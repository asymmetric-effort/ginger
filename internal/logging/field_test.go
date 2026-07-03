package logging

import (
	"errors"
	"testing"
	"time"
)

func TestStringField(t *testing.T) {
	f := String("key", "value")
	if f.Key != "key" || f.Type != FieldTypeString || f.Str != "value" {
		t.Errorf("String field: got %+v", f)
	}
}

func TestInt64Field(t *testing.T) {
	f := Int64("count", 42)
	if f.Key != "count" || f.Type != FieldTypeInt64 || f.Int != 42 {
		t.Errorf("Int64 field: got %+v", f)
	}
}

func TestIntField(t *testing.T) {
	f := Int("count", 42)
	if f.Key != "count" || f.Type != FieldTypeInt64 || f.Int != 42 {
		t.Errorf("Int field: got %+v", f)
	}
}

func TestFloat64Field(t *testing.T) {
	f := Float64("rate", 3.14)
	if f.Key != "rate" || f.Type != FieldTypeFloat64 || f.Float != 3.14 {
		t.Errorf("Float64 field: got %+v", f)
	}
}

func TestBoolField(t *testing.T) {
	f := Bool("active", true)
	if f.Key != "active" || f.Type != FieldTypeBool || !f.Bool {
		t.Errorf("Bool field: got %+v", f)
	}
}

func TestErrorField(t *testing.T) {
	err := errors.New("test error")
	f := Error(err)
	if f.Key != "error" || f.Type != FieldTypeError || f.Err != err {
		t.Errorf("Error field: got %+v", f)
	}
}

func TestNamedErrorField(t *testing.T) {
	err := errors.New("test error")
	f := NamedError("cause", err)
	if f.Key != "cause" || f.Type != FieldTypeError || f.Err != err {
		t.Errorf("NamedError field: got %+v", f)
	}
}

func TestDurationField(t *testing.T) {
	d := 5 * time.Second
	f := Duration("elapsed", d)
	if f.Key != "elapsed" || f.Type != FieldTypeDuration || f.Duration != d {
		t.Errorf("Duration field: got %+v", f)
	}
}

func TestTimeField(t *testing.T) {
	now := time.Now()
	f := Time("created", now)
	if f.Key != "created" || f.Type != FieldTypeTime || !f.TimeValue.Equal(now) {
		t.Errorf("Time field: got %+v", f)
	}
}
