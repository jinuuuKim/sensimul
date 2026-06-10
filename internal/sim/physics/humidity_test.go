package physics

import (
	"math"
	"testing"
)

// Humidity must converge to the ambient/evidence value, not to ambient+offset.
// This is the check that distinguishes "Ambient was assigned" from "Step()
// actually settles there" — the property the weather wiring depends on.
func TestHumidityConvergesToAmbient(t *testing.T) {
	e := NewHumidity(20.0)
	e.SetAmbient(67.0)
	e.NoiseSigma = 0 // remove noise so the steady state is deterministic-ish

	for i := 0; i < 1000; i++ {
		e.Step(1.0)
	}

	if math.Abs(e.Current-67.0) > 2.0 {
		t.Fatalf("humidity settled at %.2f, expected ~67.0 (evidence value)", e.Current)
	}
}

func TestTemperatureConvergesToAmbient(t *testing.T) {
	e := NewTemperature(5.0, 5.0)
	e.SetAmbient(24.5)
	e.NoiseSigma = 0

	for i := 0; i < 1000; i++ {
		e.Step(1.0)
	}

	if math.Abs(e.Current-24.5) > 2.0 {
		t.Fatalf("temperature settled at %.2f, expected ~24.5 (evidence value)", e.Current)
	}
}
