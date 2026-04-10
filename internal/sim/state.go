package sim

import (
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/sim/physics"
)

type State struct {
	Site        *domain.Site
	Sensors     []*domain.Sensor
	Controllers []*domain.Controller

	TempEngine     *physics.TemperatureEngine
	HumidityEngine *physics.HumidityEngine
	Particulate    *physics.ParticulateEngine

	Weather *WeatherSnapshot
	RNG     *Rand

	TickCount uint64
	Frozen    bool
}

type WeatherSnapshot struct {
	TemperatureC float64
	HumidityPct  float64
	PressureHPA  float64
	WindSpeedMPS float64
	Source       string
}

func NewState(site *domain.Site, seed int64) *State {
	rng := NewRand(seed)

	s := &State{
		Site:           site,
		Sensors:        make([]*domain.Sensor, 0),
		Controllers:    make([]*domain.Controller, 0),
		TempEngine:     physics.NewTemperature(site.Env.TemperatureC, site.Env.TemperatureC),
		HumidityEngine: physics.NewHumidity(site.Env.HumidityPct),
		Particulate:    physics.NewParticulate(site.Env.PM25UgM3, site.Env.PM10UgM3),
		RNG:            rng,
		TickCount:      0,
		Frozen:         false,
	}
	return s
}

func (s *State) AddSensor(sensor *domain.Sensor) {
	s.Sensors = append(s.Sensors, sensor)
}

func (s *State) AddController(ctrl *domain.Controller) {
	s.Controllers = append(s.Controllers, ctrl)
}

func (s *State) UpdateSensors(sensors []domain.Sensor) {
	newSensors := make([]*domain.Sensor, len(sensors))
	for i := range sensors {
		sensor := sensors[i]
		newSensors[i] = &sensor
	}
	s.Sensors = newSensors
}

func (s *State) UpdateControllers(controllers []domain.Controller) {
	newControllers := make([]*domain.Controller, len(controllers))
	for i := range controllers {
		ctrl := controllers[i]
		newControllers[i] = &ctrl
	}
	s.Controllers = newControllers
}

func (s *State) AdvanceTick() {
	if s.Frozen {
		return
	}
	s.TickCount++
}

func (s *State) Freeze() {
	s.Frozen = true
}

func (s *State) Unfreeze() {
	s.Frozen = false
}
