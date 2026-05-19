package web

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/sensimul/sensimul/internal/config"
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/logging"
	"github.com/sensimul/sensimul/internal/mqtt"
	"github.com/sensimul/sensimul/internal/payload"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
)

//go:embed assets/templates/*.html assets/static/* assets/docs/*.md
var assets embed.FS

// Server hosts the standalone user-facing web client service.
type Server struct {
	cfg         *config.Config
	repo        *sqlite.Repository
	logger      zerolog.Logger
	mqttClient  *mqtt.Publisher
	mqttReady   bool
	hub         *LiveHub
	httpServer  *http.Server
	templates   *template.Template
	manualText  string
	mqttDocText string
	staleAfter  time.Duration
}

type viewData struct {
	Title               string
	Path                string
	Error               string
	Success             string
	Sites               []domain.Site
	Site                *domain.Site
	Sensors             []domain.Sensor
	Sensor              *domain.Sensor
	Controllers         []domain.Controller
	Controller          *domain.Controller
	Live                []SensorLiveReading
	LiveOne             *SensorLiveReading
	StaleAfter          time.Duration
	MqttReady           bool
	ManualText          string
	RuntimeTickInterval string
}

func NewServer(configPath string) (*Server, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	repo, err := sqlite.New(cfg.SQLite.Path)
	if err != nil {
		return nil, err
	}

	logger := logging.NewLogger("web")
	tpls, err := parseTemplates()
	if err != nil {
		repo.Close()
		return nil, err
	}

	manualText, _ := readAssetText("assets/docs/user-manual.md")
	mqttText, _ := readAssetText("assets/docs/mqtt-manual.md")

	s := &Server{
		cfg:         cfg,
		repo:        repo,
		logger:      logger,
		hub:         NewLiveHub(cfg.Web.SSEBuffer),
		templates:   tpls,
		manualText:  manualText,
		mqttDocText: mqttText,
		staleAfter:  cfg.Web.StaleAfter,
	}

	s.setupMQTT()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSubFS("assets/static")))))
	mux.HandleFunc("/", s.route)

	s.httpServer = &http.Server{
		Addr:         cfg.Web.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

func (s *Server) setupMQTT() {
	client := mqtt.NewPublisher(mqtt.Options{
		BrokerURL: s.cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("%s-web", s.cfg.MQTT.ClientID),
		QoS:       s.cfg.MQTT.QoS,
		Retain:    false,
	}, logging.NewLogger("web-mqtt"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("mqtt not available - live features degraded")
		s.mqttReady = false
		return
	}

	if err := client.Subscribe(context.Background(), mqtt.TopicLiveSensorFilter(), func(topic string, body []byte) {
		parsed, err := payload.FromJSON(body)
		if err != nil {
			s.logger.Warn().Err(err).Str("topic", topic).Msg("invalid live payload")
			return
		}
		s.hub.UpsertPayload(parsed)
	}); err != nil {
		s.logger.Warn().Err(err).Msg("failed to subscribe live topics")
	}

	if err := client.Subscribe(context.Background(), mqtt.TopicTestResultFilter(), func(topic string, body []byte) {
		kind, _, _, ok := mqtt.ParseTestTopic(topic)
		if !ok || kind != "results" {
			return
		}
		var result mqtt.SensorTestResult
		if err := json.Unmarshal(body, &result); err != nil {
			s.logger.Warn().Err(err).Str("topic", topic).Msg("invalid test result payload")
			return
		}
		s.hub.PublishTest(result)
	}); err != nil {
		s.logger.Warn().Err(err).Msg("failed to subscribe test result topics")
	}

	s.mqttClient = client
	s.mqttReady = true
}

func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info().Str("addr", s.cfg.Web.ListenAddr).Msg("web server starting")
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) Close() {
	if s.mqttClient != nil {
		s.mqttClient.Close()
	}
	if s.repo != nil {
		_ = s.repo.Close()
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	if err := s.repo.DB().Ping(); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := splitPath(path)

	switch {
	case path == "":
		http.Redirect(w, r, "/sites", http.StatusSeeOther)
	case path == "sites" && r.Method == http.MethodGet:
		s.handleSitesPage(w, r)
	case path == "sites" && r.Method == http.MethodPost:
		s.handleSiteCreate(w, r)
	case len(parts) == 3 && parts[0] == "sites" && parts[2] == "edit" && r.Method == http.MethodGet:
		s.handleSiteEditPage(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "sites" && parts[2] == "edit" && r.Method == http.MethodPost:
		s.handleSiteUpdate(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "sites" && parts[2] == "delete" && r.Method == http.MethodPost:
		s.handleSiteDelete(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "sites" && parts[2] == "sensors" && r.Method == http.MethodGet:
		s.handleSensorsPage(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "sites" && parts[2] == "sensors" && r.Method == http.MethodPost:
		s.handleSensorCreate(w, r, parts[1])
	case len(parts) == 5 && parts[0] == "sites" && parts[2] == "sensors" && parts[4] == "edit" && r.Method == http.MethodGet:
		s.handleSensorEditPage(w, r, parts[1], parts[3])
	case len(parts) == 5 && parts[0] == "sites" && parts[2] == "sensors" && parts[4] == "edit" && r.Method == http.MethodPost:
		s.handleSensorUpdate(w, r, parts[1], parts[3])
	case len(parts) == 5 && parts[0] == "sites" && parts[2] == "sensors" && parts[4] == "delete" && r.Method == http.MethodPost:
		s.handleSensorDelete(w, r, parts[1], parts[3])
	case len(parts) == 3 && parts[0] == "sites" && parts[2] == "controllers" && r.Method == http.MethodGet:
		s.handleControllersPage(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "sites" && parts[2] == "controllers" && r.Method == http.MethodPost:
		s.handleControllerCreate(w, r, parts[1])
	case len(parts) == 5 && parts[0] == "sites" && parts[2] == "controllers" && parts[4] == "edit" && r.Method == http.MethodGet:
		s.handleControllerEditPage(w, r, parts[1], parts[3])
	case len(parts) == 5 && parts[0] == "sites" && parts[2] == "controllers" && parts[4] == "edit" && r.Method == http.MethodPost:
		s.handleControllerUpdate(w, r, parts[1], parts[3])
	case len(parts) == 5 && parts[0] == "sites" && parts[2] == "controllers" && parts[4] == "delete" && r.Method == http.MethodPost:
		s.handleControllerDelete(w, r, parts[1], parts[3])
	case path == "live" && r.Method == http.MethodGet:
		s.handleLivePage(w, r)
	case path == "settings/tick-interval" && r.Method == http.MethodPost:
		s.handleTickIntervalUpdate(w, r)
	case len(parts) == 3 && parts[0] == "live" && parts[1] == "sensors" && r.Method == http.MethodGet:
		s.handleLiveSensorPage(w, r, parts[2])
	case len(parts) == 4 && parts[0] == "live" && parts[1] == "sensors" && parts[3] == "test" && r.Method == http.MethodPost:
		s.handleSensorTest(w, r, parts[2])
	case path == "events/live" && r.Method == http.MethodGet:
		s.handleLiveEvents(w, r)
	case len(parts) == 3 && parts[0] == "events" && parts[1] == "tests" && r.Method == http.MethodGet:
		s.handleTestEvents(w, r, parts[2])
	case path == "docs/manual" && r.Method == http.MethodGet:
		s.render(w, "docs", viewData{Title: "User Manual", Path: path, ManualText: s.manualText, MqttReady: s.mqttReady})
	case path == "docs/mqtt" && r.Method == http.MethodGet:
		s.render(w, "docs", viewData{Title: "MQTT Manual", Path: path, ManualText: s.mqttDocText, MqttReady: s.mqttReady})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleSitesPage(w http.ResponseWriter, r *http.Request) {
	sites, err := s.repo.ListSites()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "sites", viewData{Title: "Sites", Path: strings.Trim(r.URL.Path, "/"), Sites: sites, MqttReady: s.mqttReady})
}

func (s *Server) handleSiteCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
	lon, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
	site := domain.NewSite(r.FormValue("id"), r.FormValue("name"), domain.SiteType(r.FormValue("type")), lat, lon)
	site.Timezone = valueOrDefault(r.FormValue("timezone"), "UTC")
	elevation, _ := strconv.ParseFloat(r.FormValue("elevation"), 64)
	site.Elevation = elevation

	if err := s.repo.CreateSite(site); err != nil {
		sites, _ := s.repo.ListSites()
		s.render(w, "sites", viewData{Title: "Sites", Path: "sites", Sites: sites, Error: err.Error(), MqttReady: s.mqttReady})
		return
	}

	http.Redirect(w, r, "/sites", http.StatusSeeOther)
}

func (s *Server) handleSiteEditPage(w http.ResponseWriter, r *http.Request, siteID string) {
	site, err := s.repo.GetSite(siteID)
	if err != nil || site == nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "site-edit", viewData{Title: "Edit Site", Path: "sites", Site: site, MqttReady: s.mqttReady})
}

func (s *Server) handleSiteUpdate(w http.ResponseWriter, r *http.Request, siteID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	current, err := s.repo.GetSite(siteID)
	if err != nil || current == nil {
		http.NotFound(w, r)
		return
	}

	lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
	lon, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
	elevation, _ := strconv.ParseFloat(r.FormValue("elevation"), 64)

	updated := *current
	updated.Name = r.FormValue("name")
	updated.Type = domain.SiteType(r.FormValue("type"))
	updated.Latitude = lat
	updated.Longitude = lon
	updated.Timezone = valueOrDefault(r.FormValue("timezone"), "UTC")
	updated.Elevation = elevation

	if current.Type != updated.Type {
		ctrls, listErr := s.repo.ListControllers(siteID)
		if listErr == nil && len(ctrls) > 0 {
			s.render(w, "site-edit", viewData{Title: "Edit Site", Path: "sites", Site: &updated, Error: "site type cannot change while controllers exist", MqttReady: s.mqttReady})
			return
		}
	}

	if err := s.repo.UpdateSite(&updated); err != nil {
		s.render(w, "site-edit", viewData{Title: "Edit Site", Path: "sites", Site: &updated, Error: err.Error(), MqttReady: s.mqttReady})
		return
	}
	http.Redirect(w, r, "/sites", http.StatusSeeOther)
}

func (s *Server) handleSiteDelete(w http.ResponseWriter, r *http.Request, siteID string) {
	if err := s.repo.DeleteSite(siteID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/sites", http.StatusSeeOther)
}

func (s *Server) handleSensorsPage(w http.ResponseWriter, r *http.Request, siteID string) {
	site, err := s.repo.GetSite(siteID)
	if err != nil || site == nil {
		http.NotFound(w, r)
		return
	}
	sensors, err := s.repo.ListSensors(siteID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "sensors", viewData{Title: "Sensors", Path: "sites", Site: site, Sensors: sensors, MqttReady: s.mqttReady})
}

func (s *Server) handleSensorCreate(w http.ResponseWriter, r *http.Request, siteID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sensor, err := domain.NewSensor(r.FormValue("id"), siteID, r.FormValue("sensor_type"))
	if err == nil {
		status := r.FormValue("status")
		if status != "" {
			sensor.Status = domain.SensorStatus(status)
		}
	}
	if err == nil {
		err = s.repo.CreateSensor(sensor)
	}

	if err != nil {
		site, _ := s.repo.GetSite(siteID)
		sensors, _ := s.repo.ListSensors(siteID)
		s.render(w, "sensors", viewData{Title: "Sensors", Path: "sites", Site: site, Sensors: sensors, Error: err.Error(), MqttReady: s.mqttReady})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sites/%s/sensors", siteID), http.StatusSeeOther)
}

func (s *Server) handleSensorEditPage(w http.ResponseWriter, r *http.Request, siteID, sensorID string) {
	sensor, err := s.repo.GetSensor(sensorID)
	if err != nil || sensor == nil || sensor.SiteID != siteID {
		http.NotFound(w, r)
		return
	}
	s.render(w, "sensor-edit", viewData{Title: "Edit Sensor", Path: "sites", Sensor: sensor, MqttReady: s.mqttReady})
}

func (s *Server) handleSensorUpdate(w http.ResponseWriter, r *http.Request, siteID, sensorID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sensor, err := s.repo.GetSensor(sensorID)
	if err != nil || sensor == nil || sensor.SiteID != siteID {
		http.NotFound(w, r)
		return
	}

	calibration, _ := strconv.ParseFloat(r.FormValue("calibration"), 64)
	noise, _ := strconv.ParseFloat(r.FormValue("noise_sigma"), 64)
	status := r.FormValue("status")

	sensor.Calibration = calibration
	sensor.NoiseSigma = noise
	if status != "" {
		sensor.Status = domain.SensorStatus(status)
	}

	if err := s.repo.UpdateSensor(sensor); err != nil {
		s.render(w, "sensor-edit", viewData{Title: "Edit Sensor", Path: "sites", Sensor: sensor, Error: err.Error(), MqttReady: s.mqttReady})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sites/%s/sensors", siteID), http.StatusSeeOther)
}

func (s *Server) handleSensorDelete(w http.ResponseWriter, r *http.Request, siteID, sensorID string) {
	if err := s.repo.DeleteSensor(sensorID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sites/%s/sensors", siteID), http.StatusSeeOther)
}

func (s *Server) handleControllersPage(w http.ResponseWriter, r *http.Request, siteID string) {
	site, err := s.repo.GetSite(siteID)
	if err != nil || site == nil {
		http.NotFound(w, r)
		return
	}
	controllers, err := s.repo.ListControllers(siteID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "controllers", viewData{Title: "Controllers", Path: "sites", Site: site, Controllers: controllers, MqttReady: s.mqttReady})
}

func (s *Server) handleControllerCreate(w http.ResponseWriter, r *http.Request, siteID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	site, err := s.repo.GetSite(siteID)
	if err != nil || site == nil {
		http.NotFound(w, r)
		return
	}

	ctrl, err := domain.NewController(r.FormValue("id"), siteID, domain.ControllerType(r.FormValue("type")), site.Type)
	if err == nil {
		err = s.repo.CreateController(ctrl)
	}

	if err != nil {
		controllers, _ := s.repo.ListControllers(siteID)
		s.render(w, "controllers", viewData{Title: "Controllers", Path: "sites", Site: site, Controllers: controllers, Error: err.Error(), MqttReady: s.mqttReady})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sites/%s/controllers", siteID), http.StatusSeeOther)
}

func (s *Server) handleControllerEditPage(w http.ResponseWriter, r *http.Request, siteID, controllerID string) {
	ctrl, err := s.repo.GetController(controllerID)
	if err != nil || ctrl == nil || ctrl.SiteID != siteID {
		http.NotFound(w, r)
		return
	}
	s.render(w, "controller-edit", viewData{Title: "Edit Controller", Path: "sites", Controller: ctrl, MqttReady: s.mqttReady})
}

func (s *Server) handleControllerUpdate(w http.ResponseWriter, r *http.Request, siteID, controllerID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctrl, err := s.repo.GetController(controllerID)
	if err != nil || ctrl == nil || ctrl.SiteID != siteID {
		http.NotFound(w, r)
		return
	}

	level, _ := strconv.Atoi(r.FormValue("output_level"))
	status := r.FormValue("status")
	ctrl.OutputLevel = level
	if status != "" {
		ctrl.Status = domain.ControllerStatus(status)
	}

	if err := s.repo.UpdateController(ctrl); err != nil {
		s.render(w, "controller-edit", viewData{Title: "Edit Controller", Path: "sites", Controller: ctrl, Error: err.Error(), MqttReady: s.mqttReady})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sites/%s/controllers", siteID), http.StatusSeeOther)
}

func (s *Server) handleControllerDelete(w http.ResponseWriter, r *http.Request, siteID, controllerID string) {
	if err := s.repo.DeleteController(controllerID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sites/%s/controllers", siteID), http.StatusSeeOther)
}

func (s *Server) handleLivePage(w http.ResponseWriter, r *http.Request) {
	readings := s.hub.Readings()
	sort.Slice(readings, func(i, j int) bool { return readings[i].SensorID < readings[j].SensorID })

	interval, err := s.runtimeTickInterval()
	if err != nil {
		s.render(w, "live", viewData{Title: "Live Sensors", Path: strings.Trim(r.URL.Path, "/"), Live: markStale(readings, s.staleAfter), StaleAfter: s.staleAfter, Error: err.Error(), MqttReady: s.mqttReady})
		return
	}

	success := ""
	if r.URL.Query().Get("saved") == "1" {
		success = "Tick / publish interval updated"
	}

	s.render(w, "live", viewData{Title: "Live Sensors", Path: strings.Trim(r.URL.Path, "/"), Live: markStale(readings, s.staleAfter), StaleAfter: s.staleAfter, RuntimeTickInterval: interval.String(), Success: success, MqttReady: s.mqttReady})
}

func (s *Server) handleTickIntervalUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	interval, err := time.ParseDuration(strings.TrimSpace(r.FormValue("tick_interval")))
	if err != nil || interval <= 0 {
		readings := s.hub.Readings()
		sort.Slice(readings, func(i, j int) bool { return readings[i].SensorID < readings[j].SensorID })
		s.render(w, "live", viewData{Title: "Live Sensors", Path: "live", Live: markStale(readings, s.staleAfter), StaleAfter: s.staleAfter, RuntimeTickInterval: r.FormValue("tick_interval"), Error: "tick interval must be a positive duration such as 500ms, 1s, or 5s", MqttReady: s.mqttReady})
		return
	}

	if err := s.repo.SetRuntimeDuration(sqlite.RuntimeSettingTickInterval, interval); err != nil {
		readings := s.hub.Readings()
		sort.Slice(readings, func(i, j int) bool { return readings[i].SensorID < readings[j].SensorID })
		s.render(w, "live", viewData{Title: "Live Sensors", Path: "live", Live: markStale(readings, s.staleAfter), StaleAfter: s.staleAfter, RuntimeTickInterval: interval.String(), Error: err.Error(), MqttReady: s.mqttReady})
		return
	}

	http.Redirect(w, r, "/live?saved=1", http.StatusSeeOther)
}

func (s *Server) runtimeTickInterval() (time.Duration, error) {
	interval := s.cfg.TickInterval
	if s.repo != nil {
		configured, ok, err := s.repo.GetRuntimeDuration(sqlite.RuntimeSettingTickInterval)
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

func (s *Server) handleLiveSensorPage(w http.ResponseWriter, r *http.Request, sensorID string) {
	sensor, err := s.repo.GetSensor(sensorID)
	if err != nil || sensor == nil {
		http.NotFound(w, r)
		return
	}
	reading, ok := s.hub.Reading(sensorID)
	if !ok {
		reading = SensorLiveReading{
			SiteID:     sensor.SiteID,
			SensorID:   sensor.ID,
			SensorType: sensor.SensorType,
			Unit:       sensor.Unit,
			Status:     string(sensor.Status),
		}
	}
	staled := markStale([]SensorLiveReading{reading}, s.staleAfter)
	one := staled[0]
	s.render(w, "live-sensor", viewData{Title: "Sensor Live", Path: "live", Sensor: sensor, LiveOne: &one, StaleAfter: s.staleAfter, MqttReady: s.mqttReady})
}

func (s *Server) handleSensorTest(w http.ResponseWriter, r *http.Request, sensorID string) {
	sensor, err := s.repo.GetSensor(sensorID)
	if err != nil || sensor == nil {
		http.NotFound(w, r)
		return
	}
	if s.mqttClient == nil || !s.mqttReady {
		http.Error(w, "mqtt unavailable", http.StatusServiceUnavailable)
		return
	}

	req := mqtt.NewSensorTestRequest(sensor.SiteID, sensor.ID)
	if err := s.mqttClient.PublishTestRequest(r.Context(), req); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("accepted"))
}

func (s *Server) handleLiveEvents(w http.ResponseWriter, r *http.Request) {
	s.sse(w, r, s.hub.SubscribeLive)
}

func (s *Server) handleTestEvents(w http.ResponseWriter, r *http.Request, sensorID string) {
	s.sse(w, r, func() (chan []byte, func()) { return s.hub.SubscribeTest(sensorID) })
}

func (s *Server) sse(w http.ResponseWriter, r *http.Request, subscribe func() (chan []byte, func())) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch, cancel := subscribe()
	defer cancel()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case body := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
			flusher.Flush()
		}
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data viewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseTemplates() (*template.Template, error) {
	root, err := fs.Sub(assets, "assets/templates")
	if err != nil {
		return nil, err
	}
	return template.ParseFS(root, "*.html")
}

func readAssetText(path string) (string, error) {
	b, err := assets.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func mustSubFS(path string) fs.FS {
	sub, err := fs.Sub(assets, path)
	if err != nil {
		panic(err)
	}
	return sub
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func markStale(readings []SensorLiveReading, staleAfter time.Duration) []SensorLiveReading {
	out := make([]SensorLiveReading, 0, len(readings))
	now := time.Now().UTC()
	for _, r := range readings {
		copyReading := r
		if !copyReading.LastUpdated.IsZero() && now.Sub(copyReading.LastUpdated) > staleAfter {
			copyReading.Status = "stale"
		}
		out = append(out, copyReading)
	}
	return out
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
