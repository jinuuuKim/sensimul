package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sensimul/sensimul/internal/clock"
	"github.com/sensimul/sensimul/internal/config"
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/logging"
	"github.com/sensimul/sensimul/internal/mqtt"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
	"github.com/sensimul/sensimul/internal/sim"
	"github.com/sensimul/sensimul/internal/weather"
)

// App wires runtime dependencies for CLI operations.
type App struct {
	Config    *config.Config
	Repo      *sqlite.Repository
	Publisher *mqtt.Publisher
	Weather   *weather.Client
	Logger    zerolog.Logger
}

func New(configPath string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, domain.NewConfigError("failed to load configuration", err)
	}

	repo, err := sqlite.New(cfg.SQLite.Path)
	if err != nil {
		return nil, domain.NewRuntimeError("failed to initialize sqlite repository", err)
	}

	appLogger := logging.NewLogger("app")
	publisher := mqtt.NewPublisher(
		mqtt.Options{
			BrokerURL: cfg.MQTT.BrokerURL,
			ClientID:  cfg.MQTT.ClientID,
			QoS:       cfg.MQTT.QoS,
			Retain:    cfg.MQTT.Retain,
		},
		logging.NewLogger("mqtt"),
	)

	weatherClient := weather.NewClient(cfg.Weather.Mode, cfg.Weather.APIKey, cfg.Weather.TTL)
	if err := weatherClient.Validate(); err != nil {
		repo.Close()
		return nil, domain.NewConfigError("invalid weather configuration", err)
	}

	return &App{
		Config:    cfg,
		Repo:      repo,
		Publisher: publisher,
		Weather:   weatherClient,
		Logger:    appLogger,
	}, nil
}

func (a *App) Close() {
	if a.Publisher != nil {
		a.Publisher.Close()
	}
	if a.Repo != nil {
		_ = a.Repo.Close()
	}
}

func (a *App) Health(ctx context.Context) error {
	if a.Repo == nil {
		return domain.NewRuntimeError("repository is not initialized", nil)
	}
	if err := a.Repo.DB().PingContext(ctx); err != nil {
		return domain.NewRuntimeError("sqlite health check failed", err)
	}

	if a.Publisher != nil {
		if err := a.Publisher.Connect(ctx); err != nil {
			return domain.NewExternalError("mqtt health check failed", err)
		}
	}

	return nil
}

func (a *App) Run(ctx context.Context) error {
	site, err := a.bootstrapSite()
	if err != nil {
		return err
	}

	sensors, err := a.bootstrapSensors(site.ID)
	if err != nil {
		return err
	}

	controllers, err := a.bootstrapControllers(site)
	if err != nil {
		return err
	}

	if a.Publisher != nil {
		if err := a.Publisher.Connect(ctx); err != nil {
			a.Logger.Warn().Err(err).Msg("mqtt unavailable - simulation will continue without publish")
			a.Publisher = nil
		}
	}

	state := sim.NewState(site, a.Config.Seed)
	for i := range sensors {
		sensor := sensors[i]
		state.AddSensor(&sensor)
	}
	for i := range controllers {
		controller := controllers[i]
		state.AddController(&controller)
	}

	loop := sim.NewLoop(
		state,
		sim.LoopConfig{
			TickInterval: a.Config.TickInterval,
			WeatherTTL:   a.Config.Weather.TTL,
			Seed:         a.Config.Seed,
		},
		clock.NewReal(a.Config.TickInterval),
		a.Publisher,
		a.Weather,
		a.Repo,
		site.ID,
	)

	a.Logger.Info().
		Str("site_id", site.ID).
		Dur("tick_interval", a.Config.TickInterval).
		Msg("starting simulation")

	if err := loop.Start(ctx); err != nil {
		return domain.NewRuntimeError("simulation loop failed", err)
	}

	return nil
}

func (a *App) bootstrapSite() (*domain.Site, error) {
	if a.Config.SiteID != "" {
		site, err := a.Repo.GetSite(a.Config.SiteID)
		if err != nil {
			return nil, domain.NewRuntimeError("failed to load configured site", err)
		}
		if site == nil {
			site = domain.NewSite(a.Config.SiteID, defaultSiteName(a.Config.SiteID), domain.SiteTypeIndoor, 37.5665, 126.9780)
			site.Timezone = "Asia/Seoul"
			if err := a.Repo.CreateSite(site); err != nil {
				return nil, domain.NewRuntimeError("failed to create configured site", err)
			}
		}
		return site, nil
	}

	sites, err := a.Repo.ListSites()
	if err != nil {
		return nil, domain.NewRuntimeError("failed to list sites", err)
	}
	if len(sites) > 0 {
		site := sites[0]
		return &site, nil
	}

	defaultID := fmt.Sprintf("default-%d", time.Now().Unix())
	site := domain.NewSite(defaultID, "Default Site", domain.SiteTypeIndoor, 37.5665, 126.9780)
	if err := a.Repo.CreateSite(site); err != nil {
		return nil, domain.NewRuntimeError("failed to create default site", err)
	}
	return site, nil
}

func (a *App) bootstrapSensors(siteID string) ([]domain.Sensor, error) {
	sensors, err := a.Repo.ListSensors(siteID)
	if err != nil {
		return nil, domain.NewRuntimeError("failed to load sensors", err)
	}
	if len(sensors) > 0 {
		return sensors, nil
	}

	defaults := defaultSensorDefinitions(siteID)

	for _, item := range defaults {
		sensor, err := domain.NewSensor(item.id, siteID, item.typeName)
		if err != nil {
			return nil, domain.NewRuntimeError("failed to create default sensor", err)
		}
		if err := a.Repo.CreateSensor(sensor); err != nil {
			return nil, domain.NewRuntimeError("failed to persist default sensor", err)
		}
	}

	return a.Repo.ListSensors(siteID)
}

func (a *App) bootstrapControllers(site *domain.Site) ([]domain.Controller, error) {
	controllers, err := a.Repo.ListControllers(site.ID)
	if err != nil {
		return nil, domain.NewRuntimeError("failed to load controllers", err)
	}
	if len(controllers) > 0 || !site.SupportsControllers() {
		return controllers, nil
	}

	for _, item := range defaultControllerDefinitions(site.ID) {
		controller, err := domain.NewController(item.id, site.ID, item.controllerType, site.Type)
		if err != nil {
			return nil, domain.NewRuntimeError("failed to create default controller", err)
		}
		if err := a.Repo.CreateController(controller); err != nil {
			return nil, domain.NewRuntimeError("failed to persist default controller", err)
		}
	}

	return a.Repo.ListControllers(site.ID)
}

func defaultSiteName(siteID string) string {
	if siteID == "SEOUL_COLD_CHAIN_01" {
		return "서울 냉장 물류센터 A동"
	}
	return fmt.Sprintf("Configured Site %s", siteID)
}

func defaultSensorDefinitions(siteID string) []struct {
	id       string
	typeName string
} {
	if siteID == "SEOUL_COLD_CHAIN_01" {
		return []struct {
			id       string
			typeName string
		}{
			{id: "TEMP_A01", typeName: "temperature"},
			{id: "HUM_A01", typeName: "humidity"},
			{id: "PM25_A01", typeName: "pm25"},
			{id: "DOOR_A01", typeName: "door_open"},
			{id: "MOTION_A01", typeName: "presence_detected"},
		}
	}

	return []struct {
		id       string
		typeName string
	}{
		{id: "TEMP_001", typeName: "temperature"},
		{id: "HUM_001", typeName: "humidity"},
	}
}

func defaultControllerDefinitions(siteID string) []struct {
	id             string
	controllerType domain.ControllerType
} {
	if siteID == "SEOUL_COLD_CHAIN_01" {
		return []struct {
			id             string
			controllerType domain.ControllerType
		}{
			{id: "COOL_A01", controllerType: domain.Cooling},
			{id: "DEHUM_A01", controllerType: domain.Dehumidifying},
			{id: "PURIFIER_A01", controllerType: domain.AirPurifier},
		}
	}

	return nil
}
