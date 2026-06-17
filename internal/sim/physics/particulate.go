package physics

import (
	"math"
	"math/rand"
)

// ParticulateEngine relaxes PM2.5/PM10 toward an ambient/base value, mirroring
// the temperature and humidity engines. The ambient and relaxation rate are set
// by the loop each tick from the site type and air-quality controller state:
//
//   - air purifier on  → ambient = clean baseline (fast rate)
//   - ventilation on    → ambient = outdoor/evidence PM (fast rate)
//   - all off, indoor   → ambient = outdoor PM − offset (slow rate)
//   - all off, outdoor  → ambient = outdoor/evidence PM (wind-boosted rate)
type ParticulateEngine struct {
	PM25        float64
	PM10        float64
	AmbientPM25 float64
	AmbientPM10 float64
	K           float64
	// NoiseSigma is the process-noise amplitude that turns the pure relaxation
	// into a mean-reverting random walk (OU process). Without it PM converges to
	// a constant ambient and the graph flatlines — real PM2.5/PM10 drift
	// continuously, so we inject a small per-step perturbation. PM10 swings wider
	// than PM2.5 in absolute terms, scaled by pm10NoiseFactor.
	NoiseSigma float64
}

const pm10NoiseFactor = 1.6 // PM10 절대 변동폭이 PM2.5보다 큼

func NewParticulate(pm25, pm10 float64) *ParticulateEngine {
	return &ParticulateEngine{
		PM25:        pm25,
		PM10:        pm10,
		AmbientPM25: pm25,
		AmbientPM10: pm10,
		K:           0.05,
		NoiseSigma:  0.3, // OU 배회 진폭 (실측 사이징: dt=5s·K=0.01에서 PM2.5 std ~2.7)
	}
}

// SetAmbient sets the base PM values the engine relaxes toward.
func (e *ParticulateEngine) SetAmbient(pm25, pm10 float64) {
	e.AmbientPM25 = pm25
	e.AmbientPM10 = pm10
}

// SetRate sets the relaxation rate (per second). Larger = faster convergence.
func (e *ParticulateEngine) SetRate(k float64) {
	if k > 0 {
		e.K = k
	}
}

func (e *ParticulateEngine) Step(dt float64) {
	noise25 := (rand.Float64() - 0.5) * e.NoiseSigma * 2
	noise10 := (rand.Float64() - 0.5) * e.NoiseSigma * pm10NoiseFactor * 2

	e.PM25 += (-e.K*(e.PM25-e.AmbientPM25))*dt + noise25*dt
	e.PM10 += (-e.K*(e.PM10-e.AmbientPM10))*dt + noise10*dt
	e.PM25 = math.Max(0, e.PM25)
	e.PM10 = math.Max(0, e.PM10)
}

func (e *ParticulateEngine) Reset(pm25, pm10 float64) {
	e.PM25 = pm25
	e.PM10 = pm10
	e.AmbientPM25 = pm25
	e.AmbientPM10 = pm10
}
