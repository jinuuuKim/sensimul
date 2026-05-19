package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sensimul/sensimul/internal/domain"
	_ "modernc.org/sqlite"
)

var schemaSQL = `
CREATE TABLE IF NOT EXISTS sites (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    timezone TEXT NOT NULL,
    elevation REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS sensors (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    sensor_type TEXT NOT NULL,
    value_kind TEXT NOT NULL,
    source_channel TEXT NOT NULL,
    unit TEXT,
    status TEXT NOT NULL,
    calibration REAL NOT NULL DEFAULT 1.0,
    noise_sigma REAL NOT NULL DEFAULT 0.0,
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS controllers (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    type TEXT NOT NULL,
    target_axis TEXT NOT NULL,
    status TEXT NOT NULL,
    output_level INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS runtime_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`

const RuntimeSettingTickInterval = "tick_interval"

type Repository struct {
	db *sql.DB
}

func New(dbPath string) (*Repository, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=ON&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	r := &Repository{db: db}
	if err := r.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return r, nil
}

func (r *Repository) initSchema() error {
	_, err := r.db.Exec(schemaSQL)
	return err
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

func (r *Repository) GetRuntimeDuration(key string) (time.Duration, bool, error) {
	row := r.db.QueryRow(`SELECT value FROM runtime_settings WHERE key = ?`, key)

	var raw string
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("query runtime setting %s: %w", key, err)
	}

	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, true, fmt.Errorf("parse runtime setting %s: %w", key, err)
	}
	return duration, true, nil
}

func (r *Repository) SetRuntimeDuration(key string, duration time.Duration) error {
	if key == "" {
		return fmt.Errorf("runtime setting key cannot be empty")
	}
	if duration <= 0 {
		return fmt.Errorf("runtime setting %s must be positive", key)
	}

	_, err := r.db.Exec(
		`INSERT INTO runtime_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key,
		duration.String(),
	)
	if err != nil {
		return fmt.Errorf("upsert runtime setting %s: %w", key, err)
	}
	return nil
}

func (r *Repository) CreateSite(site *domain.Site) error {
	if err := site.Validate(); err != nil {
		return err
	}

	_, err := r.db.Exec(
		`INSERT INTO sites (id, name, type, latitude, longitude, timezone, elevation) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		site.ID, site.Name, string(site.Type), site.Latitude, site.Longitude, site.Timezone, site.Elevation,
	)
	if err != nil {
		return fmt.Errorf("insert site: %w", err)
	}
	return nil
}

func (r *Repository) GetSite(id string) (*domain.Site, error) {
	row := r.db.QueryRow(`SELECT id, name, type, latitude, longitude, timezone, elevation FROM sites WHERE id = ?`, id)

	var site domain.Site
	var siteType string
	if err := row.Scan(&site.ID, &site.Name, &siteType, &site.Latitude, &site.Longitude, &site.Timezone, &site.Elevation); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query site: %w", err)
	}

	site.Type = domain.SiteType(siteType)
	site.UpdateEnv(defaultEnvironment(site.Type))
	return &site, nil
}

func (r *Repository) ListSites() ([]domain.Site, error) {
	rows, err := r.db.Query(`SELECT id, name, type, latitude, longitude, timezone, elevation FROM sites ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Site, 0)
	for rows.Next() {
		var site domain.Site
		var siteType string
		if err := rows.Scan(&site.ID, &site.Name, &siteType, &site.Latitude, &site.Longitude, &site.Timezone, &site.Elevation); err != nil {
			return nil, fmt.Errorf("scan site: %w", err)
		}
		site.Type = domain.SiteType(siteType)
		site.UpdateEnv(defaultEnvironment(site.Type))
		items = append(items, site)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sites: %w", err)
	}

	return items, nil
}

func (r *Repository) UpdateSite(site *domain.Site) error {
	if err := site.Validate(); err != nil {
		return err
	}

	res, err := r.db.Exec(
		`UPDATE sites SET name = ?, type = ?, latitude = ?, longitude = ?, timezone = ?, elevation = ? WHERE id = ?`,
		site.Name,
		string(site.Type),
		site.Latitude,
		site.Longitude,
		site.Timezone,
		site.Elevation,
		site.ID,
	)
	if err != nil {
		return fmt.Errorf("update site: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update site rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *Repository) DeleteSite(id string) error {
	res, err := r.db.Exec(`DELETE FROM sites WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete site rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *Repository) CreateSensor(sensor *domain.Sensor) error {
	if err := sensor.Validate(); err != nil {
		return err
	}

	_, err := r.db.Exec(
		`INSERT INTO sensors (id, site_id, sensor_type, value_kind, source_channel, unit, status, calibration, noise_sigma) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sensor.ID,
		sensor.SiteID,
		sensor.SensorType,
		string(sensor.ValueKind),
		sensor.SourceChannel,
		sensor.Unit,
		string(sensor.Status),
		sensor.Calibration,
		sensor.NoiseSigma,
	)
	if err != nil {
		return fmt.Errorf("insert sensor: %w", err)
	}
	return nil
}

func (r *Repository) ListSensors(siteID string) ([]domain.Sensor, error) {
	rows, err := r.db.Query(
		`SELECT id, site_id, sensor_type, value_kind, source_channel, unit, status, calibration, noise_sigma FROM sensors WHERE site_id = ? ORDER BY id`,
		siteID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sensors: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Sensor, 0)
	for rows.Next() {
		var sensor domain.Sensor
		if err := rows.Scan(
			&sensor.ID,
			&sensor.SiteID,
			&sensor.SensorType,
			&sensor.ValueKind,
			&sensor.SourceChannel,
			&sensor.Unit,
			&sensor.Status,
			&sensor.Calibration,
			&sensor.NoiseSigma,
		); err != nil {
			return nil, fmt.Errorf("scan sensor: %w", err)
		}
		items = append(items, sensor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sensors: %w", err)
	}

	return items, nil
}

func (r *Repository) GetSensor(id string) (*domain.Sensor, error) {
	row := r.db.QueryRow(
		`SELECT id, site_id, sensor_type, value_kind, source_channel, unit, status, calibration, noise_sigma FROM sensors WHERE id = ?`,
		id,
	)

	var sensor domain.Sensor
	if err := row.Scan(
		&sensor.ID,
		&sensor.SiteID,
		&sensor.SensorType,
		&sensor.ValueKind,
		&sensor.SourceChannel,
		&sensor.Unit,
		&sensor.Status,
		&sensor.Calibration,
		&sensor.NoiseSigma,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query sensor: %w", err)
	}

	return &sensor, nil
}

func (r *Repository) UpdateSensor(sensor *domain.Sensor) error {
	if err := sensor.Validate(); err != nil {
		return err
	}

	res, err := r.db.Exec(
		`UPDATE sensors SET status = ?, calibration = ?, noise_sigma = ? WHERE id = ?`,
		string(sensor.Status),
		sensor.Calibration,
		sensor.NoiseSigma,
		sensor.ID,
	)
	if err != nil {
		return fmt.Errorf("update sensor: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update sensor rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *Repository) DeleteSensor(id string) error {
	res, err := r.db.Exec(`DELETE FROM sensors WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete sensor: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete sensor rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *Repository) CreateController(ctrl *domain.Controller) error {
	if err := ctrl.Validate(); err != nil {
		return err
	}

	_, err := r.db.Exec(
		`INSERT INTO controllers (id, site_id, type, target_axis, status, output_level) VALUES (?, ?, ?, ?, ?, ?)`,
		ctrl.ID,
		ctrl.SiteID,
		string(ctrl.Type),
		string(ctrl.TargetAxis),
		string(ctrl.Status),
		ctrl.OutputLevel,
	)
	if err != nil {
		return fmt.Errorf("insert controller: %w", err)
	}
	return nil
}

func (r *Repository) ListControllers(siteID string) ([]domain.Controller, error) {
	rows, err := r.db.Query(
		`SELECT id, site_id, type, target_axis, status, output_level FROM controllers WHERE site_id = ? ORDER BY id`,
		siteID,
	)
	if err != nil {
		return nil, fmt.Errorf("list controllers: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Controller, 0)
	for rows.Next() {
		var ctrl domain.Controller
		if err := rows.Scan(&ctrl.ID, &ctrl.SiteID, &ctrl.Type, &ctrl.TargetAxis, &ctrl.Status, &ctrl.OutputLevel); err != nil {
			return nil, fmt.Errorf("scan controller: %w", err)
		}
		items = append(items, ctrl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate controllers: %w", err)
	}

	return items, nil
}

func (r *Repository) GetController(id string) (*domain.Controller, error) {
	row := r.db.QueryRow(
		`SELECT id, site_id, type, target_axis, status, output_level FROM controllers WHERE id = ?`,
		id,
	)

	var ctrl domain.Controller
	if err := row.Scan(&ctrl.ID, &ctrl.SiteID, &ctrl.Type, &ctrl.TargetAxis, &ctrl.Status, &ctrl.OutputLevel); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query controller: %w", err)
	}

	return &ctrl, nil
}

func (r *Repository) UpdateController(ctrl *domain.Controller) error {
	if err := ctrl.Validate(); err != nil {
		return err
	}

	res, err := r.db.Exec(
		`UPDATE controllers SET status = ?, output_level = ? WHERE id = ?`,
		string(ctrl.Status),
		ctrl.OutputLevel,
		ctrl.ID,
	)
	if err != nil {
		return fmt.Errorf("update controller: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update controller rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *Repository) DeleteController(id string) error {
	res, err := r.db.Exec(`DELETE FROM controllers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete controller: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete controller rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func defaultEnvironment(siteType domain.SiteType) domain.EnvironmentState {
	if siteType == domain.SiteTypeIndoor {
		return domain.EnvironmentState{
			TemperatureC: 22,
			HumidityPct:  50,
			PM25UgM3:     15,
			PM10UgM3:     30,
			PressureHPA:  1013.25,
		}
	}

	return domain.EnvironmentState{
		TemperatureC: 20,
		HumidityPct:  60,
		PM25UgM3:     20,
		PM10UgM3:     40,
		PressureHPA:  1013.25,
	}
}
