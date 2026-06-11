package app

import (
	"context"
	"fmt"
	"sync"
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

	weatherClient := weather.NewClient(
		cfg.Weather.Mode,
		cfg.Weather.APIKey,
		cfg.Weather.BaseURL,
		cfg.Weather.Station,
		cfg.Weather.TTL,
		cfg.Weather.Timeout,
	)
	if cfg.Weather.PMMode == "kma" {
		weatherClient.ConfigurePM(cfg.Weather.PMBaseURL, cfg.Weather.PMColumn)
	}
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

// Run simulates the resolved site(s). With weather.site_id set (or only one site
// present) it runs that single site; otherwise it runs every site concurrently,
// each on its own MQTT client id and simulation loop.
func (a *App) Run(ctx context.Context) error {
	sites, err := a.resolveSites()
	if err != nil {
		return err
	}

	if len(sites) == 1 {
		return a.runSite(ctx, &sites[0], a.Publisher)
	}
	return a.runMultiSite(ctx, sites)
}

// runSite builds and runs one site's loop until ctx is done. The publisher is
// connected here; the caller owns its Close().
func (a *App) runSite(ctx context.Context, site *domain.Site, pub *mqtt.Publisher) error {
	loop, err := a.buildLoop(ctx, site, pub)
	if err != nil {
		return err
	}

	a.Logger.Info().
		Str("site_id", site.ID).
		Dur("tick_interval", a.Config.TickInterval).
		Msg("starting simulation")

	if err := loop.Start(ctx); err != nil {
		return domain.NewRuntimeError("simulation loop failed", err)
	}
	return nil
}

// runMultiSite runs every site concurrently. Loops are built serially (the only
// DB writes and MQTT connects happen here) and then started together; each gets
// its own publisher/client id so they don't evict each other on the broker.
func (a *App) runMultiSite(ctx context.Context, sites []domain.Site) error {
	type built struct {
		site *domain.Site
		loop *sim.Loop
		pub  *mqtt.Publisher
	}

	items := make([]built, 0, len(sites))
	for i := range sites {
		site := &sites[i]
		pub := a.newSitePublisher(site.ID)
		loop, err := a.buildLoop(ctx, site, pub)
		if err != nil {
			for _, it := range items {
				it.pub.Close()
			}
			pub.Close()
			return err
		}
		items = append(items, built{site: site, loop: loop, pub: pub})
	}

	a.Logger.Info().Int("sites", len(items)).Msg("starting multi-site simulation")

	var wg sync.WaitGroup
	errs := make(chan error, len(items))
	for _, it := range items {
		it := it
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer it.pub.Close()
			if err := it.loop.Start(ctx); err != nil {
				a.Logger.Error().Err(err).Str("site_id", it.site.ID).Msg("site simulation failed")
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// buildLoop bootstraps the site's sensors/controllers, connects pub (best-effort
// — on failure the site runs without publishing), and constructs the loop.
func (a *App) buildLoop(ctx context.Context, site *domain.Site, pub *mqtt.Publisher) (*sim.Loop, error) {
	sensors, err := a.bootstrapSensors(site.ID)
	if err != nil {
		return nil, err
	}

	controllers, err := a.bootstrapControllers(site)
	if err != nil {
		return nil, err
	}

	publishTo := pub
	if publishTo != nil {
		if err := publishTo.Connect(ctx); err != nil {
			a.Logger.Warn().Err(err).Str("site_id", site.ID).Msg("mqtt unavailable - site continues without publish")
			publishTo = nil
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
		publishTo,
		a.Weather,
		a.Repo,
		site.ID,
	)
	return loop, nil
}

// newSitePublisher creates a publisher with a per-site client id so concurrent
// sites do not collide on the broker (a shared client id would evict peers).
func (a *App) newSitePublisher(siteID string) *mqtt.Publisher {
	base := a.Config.MQTT.ClientID
	if base == "" {
		base = "sensimul"
	}
	return mqtt.NewPublisher(
		mqtt.Options{
			BrokerURL: a.Config.MQTT.BrokerURL,
			ClientID:  base + "-" + siteID,
			QoS:       a.Config.MQTT.QoS,
			Retain:    a.Config.MQTT.Retain,
		},
		logging.NewLogger("mqtt"),
	)
}

// resolveSites returns the site(s) to simulate: the configured site_id (created
// if missing), or all sites in the repository, or a freshly created default.
func (a *App) resolveSites() ([]domain.Site, error) {
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
		return []domain.Site{*site}, nil
	}

	sites, err := a.Repo.ListSites()
	if err != nil {
		return nil, domain.NewRuntimeError("failed to list sites", err)
	}
	if len(sites) > 0 {
		return sites, nil
	}

	defaultID := fmt.Sprintf("default-%d", time.Now().Unix())
	site := domain.NewSite(defaultID, "Default Site", domain.SiteTypeIndoor, 37.5665, 126.9780)
	if err := a.Repo.CreateSite(site); err != nil {
		return nil, domain.NewRuntimeError("failed to create default site", err)
	}
	return []domain.Site{*site}, nil
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
