package physics

import (
	"math"
)

type TemperatureEngine struct {
	Current       float64
	Ambient       float64
	K             float64
	CoolingEffect float64
	HeatingEffect float64
}

func NewTemperature(initial, ambient float64) *TemperatureEngine {
	return &TemperatureEngine{
		Current: initial,
		Ambient: ambient,
		K:       0.1,
	}
}

func (e *TemperatureEngine) SetCooling(power float64) {
	e.CoolingEffect = -math.Max(0, math.Min(power, 10))
}

func (e *TemperatureEngine) SetHeating(power float64) {
	e.HeatingEffect = math.Max(0, math.Min(power, 10))
}

func (e *TemperatureEngine) Step(dt float64) float64 {
	cooling := e.CoolingEffect
	heating := e.HeatingEffect

	e.Current += (-e.K*(e.Current-e.Ambient) + cooling + heating) * dt
	return e.Current
}

func (e *TemperatureEngine) Reset(initial float64) {
	e.Current = initial
	e.CoolingEffect = 0
	e.HeatingEffect = 0
}
