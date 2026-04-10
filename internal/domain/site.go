package domain

import (
	"fmt"
)

type SiteType string

const (
	SiteTypeIndoor  SiteType = "indoor"
	SiteTypeOutdoor SiteType = "outdoor"
)

type EnvironmentState struct {
	TemperatureC float64 `json:"temperature_c"`
	HumidityPct  float64 `json:"humidity_pct"`
	PM25UgM3     float64 `json:"pm25_ug_m3"`
	PM10UgM3     float64 `json:"pm10_ug_m3"`
	PressureHPA  float64 `json:"pressure_hpa"`
}

type StateChannels struct {
	DoorOpen         bool `json:"door_open"`
	PresenceDetected bool `json:"presence_detected"`
}

type Site struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Type      SiteType         `json:"type"`
	Latitude  float64          `json:"latitude"`
	Longitude float64          `json:"longitude"`
	Timezone  string           `json:"timezone"`
	Elevation float64          `json:"elevation"`
	Env       EnvironmentState `json:"environment_state"`
	Channels  StateChannels    `json:"state_channels"`
}

func NewSite(id, name string, siteType SiteType, lat, lon float64) *Site {
	s := &Site{
		ID:        id,
		Name:      name,
		Type:      siteType,
		Latitude:  lat,
		Longitude: lon,
		Timezone:  "UTC",
		Elevation: 0,
	}
	s.resetState()
	return s
}

func (s *Site) resetState() {
	basePressure := 1013.25
	if s.Type == SiteTypeIndoor {
		s.Env = EnvironmentState{
			TemperatureC: 22.0,
			HumidityPct:  50.0,
			PM25UgM3:     15.0,
			PM10UgM3:     30.0,
			PressureHPA:  basePressure,
		}
	} else {
		s.Env = EnvironmentState{
			TemperatureC: 20.0,
			HumidityPct:  60.0,
			PM25UgM3:     20.0,
			PM10UgM3:     40.0,
			PressureHPA:  basePressure,
		}
	}
	s.Channels = StateChannels{}
}

func (s *Site) UpdateEnv(env EnvironmentState) {
	s.Env = env
}

func (s *Site) Validate() error {
	if s.ID == "" {
		return NewValidationError("site id cannot be empty")
	}
	if s.Type != SiteTypeIndoor && s.Type != SiteTypeOutdoor {
		return fmt.Errorf("invalid site type: %s", s.Type)
	}
	if s.Latitude < -90 || s.Latitude > 90 {
		return fmt.Errorf("latitude out of range: %f", s.Latitude)
	}
	if s.Longitude < -180 || s.Longitude > 180 {
		return fmt.Errorf("longitude out of range: %f", s.Longitude)
	}
	return nil
}

func (s *Site) SupportsControllers() bool {
	return s.Type == SiteTypeIndoor
}
