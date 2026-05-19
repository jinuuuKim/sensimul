package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/sensimul/sensimul/internal/clock"
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/logging"
	"github.com/sensimul/sensimul/internal/mqtt"
	"github.com/sensimul/sensimul/internal/payload"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
	"github.com/sensimul/sensimul/internal/weather"
)

type Loop struct {
	state               *State
	clock               clock.Clock
	publisher           *mqtt.Publisher
	weatherSvc          *weather.Client
	logger              zerolog.Logger
	cfg                 LoopConfig
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	testReqCh           chan mqtt.SensorTestRequest
	repo                *sqlite.Repository
	siteID              string
	configCheckInterval time.Duration
	lastConfigCheck     time.Time
	defaultTickInterval time.Duration
}

type LoopConfig struct {
	TickInterval time.Duration
	WeatherTTL   time.Duration
	Seed         int64
}

func NewLoop(state *State, cfg LoopConfig, clk clock.Clock, publisher *mqtt.Publisher, weatherSvc *weather.Client, repo *sqlite.Repository, siteID string) *Loop {
	logger := logging.NewLogger("sim")
	return &Loop{
		state:               state,
		clock:               clk,
		publisher:           publisher,
		weatherSvc:          weatherSvc,
		logger:              logger,
		cfg:                 cfg,
		testReqCh:           make(chan mqtt.SensorTestRequest, 64),
		repo:                repo,
		siteID:              siteID,
		configCheckInterval: 2 * time.Second,
		lastConfigCheck:     time.Now(),
		defaultTickInterval: cfg.TickInterval,
	}
}

func (l *Loop) Start(ctx context.Context) error {
	l.ctx, l.cancel = context.WithCancel(ctx)

	l.logger.Info().
		Str("site_id", l.state.Site.ID).
		Dur("tick_interval", l.cfg.TickInterval).
		Int64("seed", l.cfg.Seed).
		Msg("simulation loop starting")

	if err := l.subscribeTestRequests(); err != nil {
		l.logger.Warn().Err(err).Msg("test request subscription failed")
	}

	tickInterval, err := l.runtimeTickInterval()
	if err != nil {
		return err
	}
	l.cfg.TickInterval = tickInterval

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	configTicker := time.NewTicker(l.configCheckInterval)
	defer configTicker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			l.logger.Info().Uint64("ticks", l.state.TickCount).Msg("simulation loop stopped")
			return nil
		case <-ticker.C:
			if err := l.tick(); err != nil {
				l.logger.Error().Err(err).Msg("tick error")
			}
		case <-configTicker.C:
			if err := l.reloadRuntimeTickInterval(ticker); err != nil {
				l.logger.Error().Err(err).Msg("runtime interval reload error")
			}
			if err := l.reloadConfiguration(); err != nil {
				l.logger.Error().Err(err).Msg("config reload error")
			}
		}
	}
}

func (l *Loop) runtimeTickInterval() (time.Duration, error) {
	interval := l.defaultTickInterval
	if interval <= 0 {
		interval = l.cfg.TickInterval
	}

	if l.repo != nil {
		configured, ok, err := l.repo.GetRuntimeDuration(sqlite.RuntimeSettingTickInterval)
		if err != nil {
			return 0, err
		}
		if ok {
			interval = configured
		}
	}

	if interval <= 0 {
		return 0, fmt.Errorf("tick interval must be positive")
	}
	return interval, nil
}

func (l *Loop) reloadRuntimeTickInterval(ticker *time.Ticker) error {
	interval, err := l.runtimeTickInterval()
	if err != nil {
		return err
	}
	if interval == l.cfg.TickInterval {
		return nil
	}

	ticker.Reset(interval)
	old := l.cfg.TickInterval
	l.cfg.TickInterval = interval
	l.logger.Info().Dur("old_tick_interval", old).Dur("tick_interval", interval).Msg("runtime tick interval updated")
	return nil
}

func (l *Loop) reloadConfiguration() error {
	if l.repo == nil {
		return nil
	}

	l.logger.Debug().Msg("checking for configuration updates")

	sensors, err := l.repo.ListSensors(l.siteID)
	if err != nil {
		return err
	}

	controllers, err := l.repo.ListControllers(l.siteID)
	if err != nil {
		return err
	}

	oldSensorCount := len(l.state.Sensors)
	oldControllerCount := len(l.state.Controllers)

	l.state.UpdateSensors(sensors)
	l.state.UpdateControllers(controllers)

	newSensorCount := len(l.state.Sensors)
	newControllerCount := len(l.state.Controllers)

	if oldSensorCount != newSensorCount || oldControllerCount != newControllerCount {
		l.logger.Info().
			Int("old_sensors", oldSensorCount).
			Int("new_sensors", newSensorCount).
			Int("old_controllers", oldControllerCount).
			Int("new_controllers", newControllerCount).
			Msg("configuration updated")
	}

	return nil
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
	l.processTestRequests()
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
		Int("sensors", len(l.state.Sensors)).
		Int("controllers", len(l.state.Controllers)).
		Msg("tick complete")

	return nil
}

func (l *Loop) subscribeTestRequests() error {
	if l.publisher == nil {
		return nil
	}

	return l.publisher.Subscribe(l.ctx, mqtt.TopicTestRequestFilter(), func(topic string, body []byte) {
		kind, siteID, sensorID, ok := mqtt.ParseTestTopic(topic)
		if !ok || kind != "requests" || siteID != l.state.Site.ID {
			return
		}

		var req mqtt.SensorTestRequest
		if err := json.Unmarshal(body, &req); err != nil {
			l.logger.Warn().Err(err).Str("topic", topic).Msg("invalid test request payload")
			return
		}

		if req.SiteID == "" {
			req.SiteID = siteID
		}
		if req.SensorID == "" {
			req.SensorID = sensorID
		}

		select {
		case l.testReqCh <- req:
		default:
			l.logger.Warn().Str("sensor_id", req.SensorID).Msg("test request dropped: queue full")
		}
	})
}

func (l *Loop) processTestRequests() {
	if l.publisher == nil {
		return
	}

	for {
		select {
		case req := <-l.testReqCh:
			sensor := l.findSensor(req.SensorID)
			if sensor == nil {
				continue
			}

			value, ok := l.sensorValue(sensor.SourceChannel)
			if !ok {
				continue
			}

			result := mqtt.NewSensorTestResult(
				req.SiteID,
				req.SensorID,
				sensor.SensorType,
				value,
				sensor.Unit,
				string(sensor.Status),
				l.state.TickCount,
			)

			if err := l.publisher.PublishTestResult(l.ctx, result); err != nil {
				l.logger.Warn().Err(err).Str("sensor_id", req.SensorID).Msg("failed to publish test result")
			}
		default:
			return
		}
	}
}

func (l *Loop) findSensor(sensorID string) *domain.Sensor {
	for _, sensor := range l.state.Sensors {
		if sensor.ID == sensorID {
			return sensor
		}
	}
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
