package physics

import (
	"math"
)

type HumidityEngine struct {
	Current             float64
	EvaporationRate     float64
	K                   float64
	HumidifyingEffect   float64
	DehumidifyingEffect float64
}

func NewHumidity(initial float64) *HumidityEngine {
	return &HumidityEngine{
		Current:         initial,
		EvaporationRate: 0.5,
		K:               0.05,
	}
}

func (e *HumidityEngine) SetHumidifying(power float64) {
	e.HumidifyingEffect = math.Max(0, math.Min(power, 10))
}

func (e *HumidityEngine) SetDehumidifying(power float64) {
	e.DehumidifyingEffect = -math.Max(0, math.Min(power, 10))
}

func (e *HumidityEngine) Step(dt float64) float64 {
	e.Current += (e.EvaporationRate - e.K*e.Current + e.HumidifyingEffect + e.DehumidifyingEffect) * dt
	e.Current = math.Max(0, math.Min(100, e.Current))
	return e.Current
}

func (e *HumidityEngine) Reset(initial float64) {
	e.Current = initial
	e.HumidifyingEffect = 0
	e.DehumidifyingEffect = 0
}
