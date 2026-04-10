package sim

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/sensimul/sensimul/internal/clock"
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/logging"
	"github.com/sensimul/sensimul/internal/mqtt"
	"github.com/sensimul/sensimul/internal/payload"
	"github.com/sensimul/sensimul/internal/weather"
)

type Loop struct {
	state      *State
	clock      clock.Clock
	publisher  *mqtt.Publisher
	weatherSvc *weather.Client
	logger     zerolog.Logger
	cfg        LoopConfig
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

type LoopConfig struct {
	TickInterval time.Duration
	WeatherTTL   time.Duration
	Seed         int64
}

func NewLoop(state *State, cfg LoopConfig, clk clock.Clock, publisher *mqtt.Publisher, weatherSvc *weather.Client) *Loop {
	logger := logging.NewLogger("sim")
	return &Loop{
		state:      state,
		clock:      clk,
		publisher:  publisher,
		weatherSvc: weatherSvc,
		logger:     logger,
		cfg:        cfg,
	}
}

func (l *Loop) Start(ctx context.Context) error {
	l.ctx, l.cancel = context.WithCancel(ctx)

	l.logger.Info().
		Str("site_id", l.state.Site.ID).
		Dur("tick_interval", l.cfg.TickInterval).
		Int64("seed", l.cfg.Seed).
		Msg("simulation loop starting")

	for {
		select {
		case <-l.ctx.Done():
			l.logger.Info().Uint64("ticks", l.state.TickCount).Msg("simulation loop stopped")
			return nil
		default:
			if err := l.tick(); err != nil {
				l.logger.Error().Err(err).Msg("tick error")
			}

			if l.cfg.TickInterval > 0 {
				time.Sleep(l.cfg.TickInterval)
			}
		}
	}
}

func (l *Loop) tick() error {
	if l.state.Frozen {
		l.logger.Warn().Msg("tick skipped - simulation frozen")
		return nil
	}

	l.state.AdvanceTick()

	if l.shouldRefreshWeather() {
		if err := l.refreshWeather(); err != nil {
			l.logger.Warn().Err(err).Msg("weather refresh failed")
		}
	}

	l.resolveControllers()
	dt := l.tickSeconds()

	temp := l.state.TempEngine.Step(dt)
	humidity := l.state.HumidityEngine.Step(dt)
	l.state.Particulate.Step(dt)

	l.state.Site.Env.TemperatureC = temp
	l.state.Site.Env.HumidityPct = humidity
	l.state.Site.Env.PM25UgM3 = l.state.Particulate.PM25
	l.state.Site.Env.PM10UgM3 = l.state.Particulate.PM10

	l.publishSensors()

	l.logger.Debug().
		Uint64("tick", l.state.TickCount).
		Float64("temp", l.state.TempEngine.Current).
		Float64("humidity", l.state.HumidityEngine.Current).
		Msg("tick complete")

	return nil
}

func (l *Loop) shouldRefreshWeather() bool {
	return l.state.Site.Type == domain.SiteTypeOutdoor && l.weatherSvc != nil
}

func (l *Loop) refreshWeather() error {
	w, err := l.weatherSvc.Get()
	if err != nil {
		return err
	}
	l.state.Weather = &WeatherSnapshot{
		TemperatureC: w.TemperatureC,
		HumidityPct:  w.HumidityPct,
		PressureHPA:  w.PressureHPA,
		WindSpeedMPS: w.WindSpeedMPS,
		Source:       string(w.Source),
	}
	return nil
}

func (l *Loop) resolveControllers() {
	activeControllers := make(map[domain.TargetAxis]bool)

	for _, ctrl := range l.state.Controllers {
		if ctrl.Status == domain.ControllerStatusOn && ctrl.OutputLevel > 0 {
			power := float64(ctrl.OutputLevel) / 10
			switch ctrl.Type {
			case domain.Cooling:
				l.state.TempEngine.SetCooling(power)
			case domain.Heating:
				l.state.TempEngine.SetHeating(power)
			case domain.Humidifying:
				l.state.HumidityEngine.SetHumidifying(power)
			case domain.Dehumidifying:
				l.state.HumidityEngine.SetDehumidifying(power)
			case domain.Ventilation:
				outdoorPM25, outdoorPM10 := l.outdoorParticulate()
				l.state.Particulate.ApplyVentilation(power/10, outdoorPM25, outdoorPM10)
			case domain.AirPurifier:
				l.state.Particulate.ApplyAirPurifier(power / 10)
			}
			activeControllers[ctrl.TargetAxis] = true
		}
	}

	if !activeControllers[domain.AxisTemperature] {
		l.state.TempEngine.SetCooling(0)
		l.state.TempEngine.SetHeating(0)
	}
	if !activeControllers[domain.AxisHumidity] {
		l.state.HumidityEngine.SetHumidifying(0)
		l.state.HumidityEngine.SetDehumidifying(0)
	}
}

func (l *Loop) tickSeconds() float64 {
	dt := l.cfg.TickInterval.Seconds()
	if l.clock != nil {
		if tick := l.clock.TickDuration(); tick > 0 {
			dt = tick.Seconds()
		}
	}
	if dt <= 0 {
		return 1
	}
	return dt
}

func (l *Loop) outdoorParticulate() (float64, float64) {
	if l.state.Weather == nil {
		return l.state.Site.Env.PM25UgM3, l.state.Site.Env.PM10UgM3
	}
	// Weather API does not provide PM directly in current snapshot,
	// so the last simulated site values are used as a stable baseline.
	return l.state.Site.Env.PM25UgM3, l.state.Site.Env.PM10UgM3
}

func (l *Loop) publishSensors() {
	if l.publisher == nil {
		return
	}

	pubCtx := l.ctx
	if pubCtx == nil {
		pubCtx = context.Background()
	}

	for _, sensor := range l.state.Sensors {
		value, ok := l.sensorValue(sensor.SourceChannel)
		if !ok {
			continue
		}

		if sensor.NoiseSigma > 0 {
			value += ((l.state.RNG.Float64() * 2) - 1) * sensor.NoiseSigma
		}
		value *= sensor.Calibration

		msg := payload.New(
			sensor.SiteID,
			sensor.ID,
			sensor.SensorType,
			string(sensor.ValueKind),
			value,
			sensor.Unit,
			string(sensor.Status),
			l.state.TickCount,
		)

		bytes, err := msg.ToJSON()
		if err != nil {
			l.logger.Error().Err(err).Str("sensor_id", sensor.ID).Msg("payload encode failed")
			continue
		}

		if err := l.publisher.PublishSensor(pubCtx, sensor.SiteID, sensor.ID, bytes); err != nil {
			l.logger.Warn().Err(err).Str("sensor_id", sensor.ID).Msg("publish failed")
		}
	}
}

func (l *Loop) sensorValue(channel string) (float64, bool) {
	switch channel {
	case "temperature_c":
		return l.state.Site.Env.TemperatureC, true
	case "humidity_pct":
		return l.state.Site.Env.HumidityPct, true
	case "pm25_ug_m3":
		return l.state.Site.Env.PM25UgM3, true
	case "pm10_ug_m3":
		return l.state.Site.Env.PM10UgM3, true
	case "pressure_hpa":
		return l.state.Site.Env.PressureHPA, true
	case "door_open":
		if l.state.Site.Channels.DoorOpen {
			return 1, true
		}
		return 0, true
	case "presence_detected":
		if l.state.Site.Channels.PresenceDetected {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func (l *Loop) Stop() {
	if l.cancel != nil {
		l.cancel()
	}
	l.wg.Wait()
}
