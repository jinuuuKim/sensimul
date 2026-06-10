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

// Particulate model tuning. PM ambient targets and rates set per tick.
const (
	cleanPM25 = 5.0  // 청정 기준값 (good air, purifier target)
	cleanPM10 = 10.0 // 청정 기준값

	indoorOffsetPM25 = 5.0  // 실내는 실외보다 이만큼 낮게 수렴 (off 상태)
	indoorOffsetPM10 = 10.0 // 실내는 실외보다 이만큼 낮게 수렴 (off 상태)

	pmRateBase = 0.05 // 기본 수렴 속도 (실외)
	pmRateFast = 0.30 // 디바이스 ON 시 빠른 수렴
	pmRateSlow = 0.01 // 실내 OFF 시 천천히 수렴

	humidityBaseK  = 0.05 // 습도 기본 수렴 속도 (physics.NewHumidity와 일치)
	windEvapFactor = 0.03 // 풍속 → 증발(습도 수렴) 가속 계수
	windVentFactor = 0.05 // 풍속 → 환기(미세먼지 수렴) 가속 계수
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
	weatherInitialized  bool
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
	l.resolveParticulate()
	l.applyWindEffects()
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
	// Both indoor and outdoor sites consume weather: outdoor sites run on it
	// directly, while indoor sites relax toward it whenever their controllers are
	// off (조절기 off → 기상청 값에 수렴).
	return l.weatherSvc != nil
}

// refreshWeather pulls the current weather evidence and feeds it into the
// simulation as a base value. The KMA response is not stored wholesale; only the
// engines' ambient/base values are seeded or nudged.
//
//   - Every fetch sets the engines' ambient target to the observation. With no
//     active controller on an axis the engine relaxes to this ambient, so the
//     sensor converges to the KMA value ("값 조정"). Active controllers
//     (cooling/heating/humidify/dehumidify) bias the equilibrium away from it.
//   - Outdoor sites additionally snap their CURRENT value to the first
//     observation so a freshly configured site starts from real outdoor
//     conditions ("초기 기반 값"). Indoor sites keep their configured baseline as
//     the starting point and only drift toward the observation when controllers
//     are off.
//
// Get always returns a usable snapshot; a non-nil error means a live fetch
// failed but a fallback (last-good cache / synthetic) was applied, which we log
// without discarding the value.
func (l *Loop) refreshWeather() error {
	w, err := l.weatherSvc.GetForStation(l.state.Site.WeatherStation)
	if w == nil {
		return err
	}

	l.state.Weather = &WeatherSnapshot{
		TemperatureC: w.TemperatureC,
		HumidityPct:  w.HumidityPct,
		PressureHPA:  w.PressureHPA,
		WindSpeedMPS: w.WindSpeedMPS,
		PM25UgM3:     w.PM25UgM3,
		PM10UgM3:     w.PM10UgM3,
		HasPM:        w.HasPM10,
		Source:       string(w.Source),
	}

	if l.state.Site.Type == domain.SiteTypeOutdoor && !l.weatherInitialized {
		l.state.TempEngine.Current = w.TemperatureC
		l.state.HumidityEngine.Current = w.HumidityPct
	}

	l.state.TempEngine.SetAmbient(w.TemperatureC)
	l.state.HumidityEngine.SetAmbient(w.HumidityPct)

	// Pressure has no dynamic engine; the weather observation is the base value.
	l.state.Site.Env.PressureHPA = w.PressureHPA

	l.weatherInitialized = true

	return err
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
			case domain.Ventilation, domain.AirPurifier:
				// Air-quality controllers drive PM ambient in resolveParticulate.
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

// outdoorParticulate returns the outdoor PM reference. PM10 uses the KMA 황사
// evidence when available; PM2.5 always uses the simulated baseline (KMA 황사
// provides PM10 only — PM2.5 stays simulated).
func (l *Loop) outdoorParticulate() (float64, float64) {
	pm25 := l.state.BaselinePM25
	pm10 := l.state.BaselinePM10
	if l.state.Weather != nil && l.state.Weather.HasPM {
		pm10 = l.state.Weather.PM10UgM3
	}
	return pm25, pm10
}

// resolveParticulate sets the PM ambient target and convergence rate from the
// air-quality controller state and site type. Air purifier and ventilation are
// modelled as physical opposites: a purifier cleans toward a low baseline, while
// ventilation exchanges with outside air (pulling PM toward the outdoor value).
func (l *Loop) resolveParticulate() {
	outdoorPM25, outdoorPM10 := l.outdoorParticulate()

	purifierOn := false
	ventilationOn := false
	for _, ctrl := range l.state.Controllers {
		if ctrl.Status != domain.ControllerStatusOn || ctrl.OutputLevel <= 0 {
			continue
		}
		switch ctrl.Type {
		case domain.AirPurifier:
			purifierOn = true
		case domain.Ventilation:
			ventilationOn = true
		}
	}

	switch {
	case purifierOn:
		// 공기청정기 ON → 청정 기준값으로 빠르게 수렴.
		l.state.Particulate.SetAmbient(cleanPM25, cleanPM10)
		l.state.Particulate.SetRate(pmRateFast)
	case ventilationOn:
		// 환풍기 ON → 외부 공기 유입 → 실외(황사) 값으로 빠르게 수렴.
		l.state.Particulate.SetAmbient(outdoorPM25, outdoorPM10)
		l.state.Particulate.SetRate(pmRateFast)
	case l.state.Site.Type == domain.SiteTypeOutdoor:
		// 실외: 기상청 PM 값에 수렴 (풍속이 환기를 가속).
		l.state.Particulate.SetAmbient(outdoorPM25, outdoorPM10)
		l.state.Particulate.SetRate(l.windAdjustedRate(pmRateBase, windVentFactor))
	default:
		// 실내 OFF → 천천히 (실외 - 일정값)에 수렴 (실내라 약간 더 깨끗).
		l.state.Particulate.SetAmbient(
			maxFloat(0, outdoorPM25-indoorOffsetPM25),
			maxFloat(0, outdoorPM10-indoorOffsetPM10),
		)
		l.state.Particulate.SetRate(pmRateSlow)
	}
}

// applyWindEffects connects outdoor wind speed to the evaporation (humidity)
// convergence rate. Wind speeds up mixing toward the ambient value but does not
// change the equilibrium (the KMA reading already embeds wind), so this is a
// transient effect. Indoor sites and missing weather keep the base rate.
func (l *Loop) applyWindEffects() {
	if l.state.Site.Type != domain.SiteTypeOutdoor || l.state.Weather == nil {
		l.state.HumidityEngine.K = humidityBaseK
		return
	}
	l.state.HumidityEngine.K = humidityBaseK * windFactor(l.state.Weather.WindSpeedMPS, windEvapFactor)
}

func (l *Loop) windAdjustedRate(base, factor float64) float64 {
	if l.state.Weather == nil {
		return base
	}
	return base * windFactor(l.state.Weather.WindSpeedMPS, factor)
}

func windFactor(windMPS, factor float64) float64 {
	if windMPS <= 0 {
		return 1
	}
	return 1 + factor*windMPS
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
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
