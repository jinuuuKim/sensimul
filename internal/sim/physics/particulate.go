package physics

import (
	"math"
)

type ParticulateEngine struct {
	PM25           float64
	PM10           float64
	DepositionRate float64
}

func NewParticulate(pm25, pm10 float64) *ParticulateEngine {
	return &ParticulateEngine{
		PM25:           pm25,
		PM10:           pm10,
		DepositionRate: 0.1,
	}
}

func (e *ParticulateEngine) ApplyVentilation(inflowRate float64, outdoorPM25, outdoorPM10 float64) {
	if inflowRate > 0 {
		e.PM25 = e.PM25*(1-inflowRate) + outdoorPM25*inflowRate
		e.PM10 = e.PM10*(1-inflowRate) + outdoorPM10*inflowRate
	}
}

func (e *ParticulateEngine) ApplyAirPurifier(efficiency float64) {
	e.PM25 *= (1 - efficiency*0.5)
	e.PM10 *= (1 - efficiency*0.3)
}

func (e *ParticulateEngine) Step(dt float64) {
	e.PM25 = math.Max(0, e.PM25-e.DepositionRate*dt)
	e.PM10 = math.Max(0, e.PM10-e.DepositionRate*0.8*dt)
}

func (e *ParticulateEngine) Reset(pm25, pm10 float64) {
	e.PM25 = pm25
	e.PM10 = pm10
}
