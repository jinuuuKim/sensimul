package domain

import "testing"

func TestControllerRequiredOutput(t *testing.T) {
	cases := []struct {
		name     string
		ctrlType ControllerType
		target   float64
		ambient  float64
		want     int
	}{
		{"cooling needs output below ambient", Cooling, 22, 30, 16}, // 2*(30-22)
		{"cooling not needed when already cool", Cooling, 22, 20, 0},
		{"heating needs output above ambient", Heating, 26, 18, 16}, // 2*(26-18)
		{"heating not needed when already warm", Heating, 26, 30, 0},
		{"humidify", Humidifying, 60, 45, 15}, // 60-45
		{"dehumidify", Dehumidifying, 40, 70, 30},
		{"clamped to 100", Cooling, 0, 90, 100}, // 2*90=180 -> 100
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Controller{Type: tc.ctrlType, TargetValue: tc.target, HasTarget: true}
			if got := c.RequiredOutput(tc.ambient); got != tc.want {
				t.Fatalf("RequiredOutput(%.1f) = %d, want %d", tc.ambient, got, tc.want)
			}
		})
	}
}

func TestControllerRequiredOutputFallsBackToManual(t *testing.T) {
	// No target set -> legacy manual output is returned verbatim.
	c := &Controller{Type: Cooling, OutputLevel: 80, HasTarget: false}
	if got := c.RequiredOutput(30); got != 80 {
		t.Fatalf("legacy RequiredOutput = %d, want 80 (manual output)", got)
	}
}
