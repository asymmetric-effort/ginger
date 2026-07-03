package sampling

import (
	"testing"
	"time"
)

func TestAdaptiveEngineBasic(t *testing.T) {
	ae := NewAdaptiveEngine(10) // target 10 QPS

	// Simulate 100 spans in 1 second — well above target
	for i := 0; i < 100; i++ {
		ae.RecordSpan("svc")
	}

	ae.Compute(time.Second)

	p := ae.GetProbability("svc")
	if p >= 1.0 {
		t.Errorf("probability should drop below 1.0 when over target, got %f", p)
	}
	if p <= 0 {
		t.Error("probability should be positive")
	}
}

func TestAdaptiveEngineUnknownService(t *testing.T) {
	ae := NewAdaptiveEngine(1)
	if ae.GetProbability("unknown") != 1.0 {
		t.Error("unknown service should have probability 1.0")
	}
}

func TestAdaptiveEngineConverges(t *testing.T) {
	ae := NewAdaptiveEngine(10)

	// Multiple compute cycles with high throughput
	for cycle := 0; cycle < 5; cycle++ {
		for i := 0; i < 100; i++ {
			ae.RecordSpan("svc")
		}
		ae.Compute(time.Second)
	}

	p := ae.GetProbability("svc")
	// Should converge toward target/observed ≈ 10/100 = 0.1
	if p > 0.5 {
		t.Errorf("should converge toward low rate, got %f", p)
	}
}

func TestAdaptiveEngineGetAllProbabilities(t *testing.T) {
	ae := NewAdaptiveEngine(1)
	ae.RecordSpan("a")
	ae.RecordSpan("b")
	ae.Compute(time.Second)

	all := ae.GetAllProbabilities()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestAdaptiveEngineDefault(t *testing.T) {
	ae := NewAdaptiveEngine(0) // should default to 1
	if ae.target != 1 {
		t.Errorf("default target = %f", ae.target)
	}
}

func TestAdaptiveEngineNoObservations(t *testing.T) {
	ae := NewAdaptiveEngine(10)
	ae.Compute(time.Second) // no spans recorded
	// Should not panic, no probabilities set
	if len(ae.GetAllProbabilities()) != 0 {
		t.Error("should have no probabilities without observations")
	}
}

func TestAdaptiveEngineZeroInterval(t *testing.T) {
	ae := NewAdaptiveEngine(10)
	for i := 0; i < 10; i++ {
		ae.RecordSpan("svc")
	}
	// Zero duration should default to 1 second to avoid divide by zero
	ae.Compute(0)
	p := ae.GetProbability("svc")
	if p <= 0 || p > 1 {
		t.Errorf("probability should be in (0,1] after zero interval, got %f", p)
	}
}

func TestAdaptiveEngineNegativeInterval(t *testing.T) {
	ae := NewAdaptiveEngine(10)
	for i := 0; i < 10; i++ {
		ae.RecordSpan("svc")
	}
	// Negative duration should also default to 1 second
	ae.Compute(-time.Second)
	p := ae.GetProbability("svc")
	if p <= 0 || p > 1 {
		t.Errorf("probability should be in (0,1] after negative interval, got %f", p)
	}
}

func TestAdaptiveEngineZeroObservedRate(t *testing.T) {
	ae := NewAdaptiveEngine(10)
	// Record a span so the window entry exists
	ae.RecordSpan("svc")
	// First compute drains the count to 0
	ae.Compute(time.Second)
	// Second compute has zero observed rate — should hit the continue branch
	ae.Compute(time.Second)
	// Should still have the probability from first compute
	p := ae.GetProbability("svc")
	if p <= 0 || p > 1 {
		t.Errorf("probability out of range: %f", p)
	}
}

func TestAdaptiveEngineExistingProbability(t *testing.T) {
	ae := NewAdaptiveEngine(10)

	// First compute creates the probability for "svc"
	for i := 0; i < 50; i++ {
		ae.RecordSpan("svc")
	}
	ae.Compute(time.Second)

	// Second compute should use existing probability (ok = true branch)
	for i := 0; i < 50; i++ {
		ae.RecordSpan("svc")
	}
	ae.Compute(time.Second)

	p := ae.GetProbability("svc")
	if p <= 0 || p > 1 {
		t.Errorf("probability out of range: %f", p)
	}
}
