package sim

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
	"github.com/sensimul/sensimul/internal/weather"
)

func TestLoopRuntimeTickIntervalUsesRepositorySetting(t *testing.T) {
	repo, err := sqlite.New(filepath.Join(t.TempDir(), "sensimul.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	if err := repo.SetRuntimeDuration(sqlite.RuntimeSettingTickInterval, 250*time.Millisecond); err != nil {
		t.Fatalf("set runtime interval: %v", err)
	}

	site := domain.NewSite("S1", "Site1", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	loop := NewLoop(state, LoopConfig{TickInterval: time.Second}, nil, nil, nil, repo, site.ID)

	got, err := loop.runtimeTickInterval()
	if err != nil {
		t.Fatalf("runtime interval: %v", err)
	}
	if got != 250*time.Millisecond {
		t.Fatalf("expected repository interval, got %s", got)
	}
}

func TestLoopRuntimeTickIntervalFallsBackToConfig(t *testing.T) {
	site := domain.NewSite("S1", "Site1", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	loop := NewLoop(state, LoopConfig{TickInterval: 3 * time.Second}, nil, nil, nil, nil, site.ID)

	got, err := loop.runtimeTickInterval()
	if err != nil {
		t.Fatalf("runtime interval: %v", err)
	}
	if got != 3*time.Second {
		t.Fatalf("expected config interval, got %s", got)
	}
}

func TestRefreshWeatherInitializesThenAdjusts(t *testing.T) {
	site := domain.NewSite("OUT1", "Outdoor", domain.SiteTypeOutdoor, 37.5, 126.9)
	state := NewState(site, 1)

	wc := weather.NewClient(weather.ModeSynthetic, "", "", "", time.Minute, time.Second)
	wc.SetCache(&weather.Weather{TemperatureC: 5.0, HumidityPct: 80.0, PressureHPA: 1004.0, WindSpeedMPS: 2.0})

	loop := NewLoop(state, LoopConfig{TickInterval: time.Second}, nil, nil, wc, nil, site.ID)

	// First refresh seeds both current and ambient (초기 기반 값).
	_ = loop.refreshWeather()
	if state.TempEngine.Current != 5.0 || state.TempEngine.Ambient != 5.0 {
		t.Fatalf("temp not initialized: current=%v ambient=%v", state.TempEngine.Current, state.TempEngine.Ambient)
	}
	if state.HumidityEngine.Ambient != 80.0 {
		t.Fatalf("humidity ambient not initialized: %v", state.HumidityEngine.Ambient)
	}
	if state.Site.Env.PressureHPA != 1004.0 {
		t.Fatalf("pressure not set: %v", state.Site.Env.PressureHPA)
	}

	// Simulate the engine drifting, then a new observation.
	state.TempEngine.Current = 12.0
	wc.SetCache(&weather.Weather{TemperatureC: 6.0, HumidityPct: 75.0, PressureHPA: 1005.0, WindSpeedMPS: 1.0})

	// Second refresh only adjusts ambient, leaving current intact (값 조정).
	_ = loop.refreshWeather()
	if state.TempEngine.Ambient != 6.0 {
		t.Fatalf("temp ambient not adjusted: %v", state.TempEngine.Ambient)
	}
	if state.TempEngine.Current != 12.0 {
		t.Fatalf("temp current must not reset on adjust: %v", state.TempEngine.Current)
	}
	if state.HumidityEngine.Ambient != 75.0 {
		t.Fatalf("humidity ambient not adjusted: %v", state.HumidityEngine.Ambient)
	}
}

func TestIndoorWeatherDrivenWhenControllersOff(t *testing.T) {
	site := domain.NewSite("IN1", "Indoor", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	state.TempEngine.NoiseSigma = 0

	wc := weather.NewClient(weather.ModeSynthetic, "", "", "", time.Minute, time.Second)
	wc.SetCache(&weather.Weather{TemperatureC: 30.0, HumidityPct: 40.0, PressureHPA: 1009.0})

	loop := NewLoop(state, LoopConfig{TickInterval: time.Second}, nil, nil, wc, nil, site.ID)

	_ = loop.refreshWeather()

	// Indoor follows weather as ambient, but does NOT snap its current value to it.
	if state.TempEngine.Ambient != 30.0 {
		t.Fatalf("indoor ambient = %v, want 30.0 (KMA value)", state.TempEngine.Ambient)
	}
	if state.TempEngine.Current == 30.0 {
		t.Fatal("indoor current must not snap to weather on init")
	}

	// Controllers off → converge to the KMA value.
	for i := 0; i < 500; i++ {
		loop.resolveControllers()
		state.TempEngine.Step(1.0)
	}
	if math.Abs(state.TempEngine.Current-30.0) > 1.0 {
		t.Fatalf("indoor temp settled at %.2f, want ~30.0 when controllers off", state.TempEngine.Current)
	}
}

func TestIndoorControllerOverridesWeather(t *testing.T) {
	site := domain.NewSite("IN2", "Indoor", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	state.TempEngine.NoiseSigma = 0

	cooler, err := domain.NewController("COOL1", site.ID, domain.Cooling, domain.SiteTypeIndoor)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	cooler.Status = domain.ControllerStatusOn
	cooler.OutputLevel = 80
	state.AddController(cooler)

	wc := weather.NewClient(weather.ModeSynthetic, "", "", "", time.Minute, time.Second)
	wc.SetCache(&weather.Weather{TemperatureC: 30.0, HumidityPct: 40.0, PressureHPA: 1009.0})

	loop := NewLoop(state, LoopConfig{TickInterval: time.Second}, nil, nil, wc, nil, site.ID)
	_ = loop.refreshWeather()

	// Cooler on → equilibrium pulled well below the 30°C weather ambient.
	for i := 0; i < 500; i++ {
		loop.resolveControllers()
		state.TempEngine.Step(1.0)
	}
	if state.TempEngine.Current >= 30.0 {
		t.Fatalf("cooler on must drive temp below weather ambient, got %.2f", state.TempEngine.Current)
	}
}

func newLoopWith(state *State, siteID string) *Loop {
	return NewLoop(state, LoopConfig{TickInterval: time.Second}, nil, nil, nil, nil, siteID)
}

func stepParticulate(loop *Loop, state *State, n int) {
	for i := 0; i < n; i++ {
		loop.resolveParticulate()
		state.Particulate.Step(1.0)
	}
}

func TestIndoorPurifierConvergesToCleanPM(t *testing.T) {
	site := domain.NewSite("IN1", "Indoor", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	state.Particulate.PM25, state.Particulate.PM10 = 120, 200 // dusty

	ap, err := domain.NewController("AP1", site.ID, domain.AirPurifier, domain.SiteTypeIndoor)
	if err != nil {
		t.Fatalf("controller: %v", err)
	}
	ap.Status, ap.OutputLevel = domain.ControllerStatusOn, 80
	state.AddController(ap)

	loop := newLoopWith(state, site.ID)
	stepParticulate(loop, state, 1000)

	if math.Abs(state.Particulate.PM25-cleanPM25) > 0.5 || math.Abs(state.Particulate.PM10-cleanPM10) > 0.5 {
		t.Fatalf("purifier on: PM=%.1f/%.1f want clean %.0f/%.0f",
			state.Particulate.PM25, state.Particulate.PM10, cleanPM25, cleanPM10)
	}
}

func TestIndoorVentilationConvergesToOutdoorPM(t *testing.T) {
	site := domain.NewSite("IN2", "Indoor", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	state.Particulate.PM25, state.Particulate.PM10 = 5, 10 // start clean
	state.Weather = &WeatherSnapshot{PM10UgM3: 150, HasPM: true}

	vent, err := domain.NewController("V1", site.ID, domain.Ventilation, domain.SiteTypeIndoor)
	if err != nil {
		t.Fatalf("controller: %v", err)
	}
	vent.Status, vent.OutputLevel = domain.ControllerStatusOn, 80
	state.AddController(vent)

	loop := newLoopWith(state, site.ID)
	stepParticulate(loop, state, 1000)

	// 환풍기 ON: PM10 pulled to outdoor (150). PM2.5 → simulated baseline.
	if math.Abs(state.Particulate.PM10-150) > 1.0 {
		t.Fatalf("ventilation on: PM10=%.1f want ~150 (outdoor)", state.Particulate.PM10)
	}
	if math.Abs(state.Particulate.PM25-state.BaselinePM25) > 1.0 {
		t.Fatalf("ventilation on: PM2.5=%.1f want baseline %.1f", state.Particulate.PM25, state.BaselinePM25)
	}
}

func TestIndoorOffConvergesToOutdoorMinusOffset(t *testing.T) {
	site := domain.NewSite("IN3", "Indoor", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	state.Weather = &WeatherSnapshot{PM10UgM3: 150, HasPM: true}

	loop := newLoopWith(state, site.ID)
	stepParticulate(loop, state, 3000) // slow rate

	wantPM10 := 150 - indoorOffsetPM10
	wantPM25 := state.BaselinePM25 - indoorOffsetPM25
	if math.Abs(state.Particulate.PM10-wantPM10) > 1.5 {
		t.Fatalf("indoor off: PM10=%.1f want ~%.1f (outdoor-offset)", state.Particulate.PM10, wantPM10)
	}
	if math.Abs(state.Particulate.PM25-wantPM25) > 1.5 {
		t.Fatalf("indoor off: PM2.5=%.1f want ~%.1f", state.Particulate.PM25, wantPM25)
	}
}

func TestWindBoostsHumidityRateOutdoorOnly(t *testing.T) {
	out := domain.NewSite("OUT9", "Outdoor", domain.SiteTypeOutdoor, 37.5, 126.9)
	state := NewState(out, 1)
	state.Weather = &WeatherSnapshot{WindSpeedMPS: 10}
	loop := newLoopWith(state, out.ID)

	loop.applyWindEffects()
	want := humidityBaseK * (1 + windEvapFactor*10)
	if math.Abs(state.HumidityEngine.K-want) > 1e-9 {
		t.Fatalf("outdoor wind: humidity K=%.4f want %.4f", state.HumidityEngine.K, want)
	}

	in := domain.NewSite("IN9", "Indoor", domain.SiteTypeIndoor, 37.5, 126.9)
	istate := NewState(in, 1)
	istate.Weather = &WeatherSnapshot{WindSpeedMPS: 10}
	iloop := newLoopWith(istate, in.ID)
	iloop.applyWindEffects()
	if math.Abs(istate.HumidityEngine.K-humidityBaseK) > 1e-9 {
		t.Fatalf("indoor wind must not change humidity K, got %.4f", istate.HumidityEngine.K)
	}
}
