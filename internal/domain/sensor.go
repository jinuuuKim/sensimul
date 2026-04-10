package domain

import (
	"fmt"
)

type ValueKind string

const (
	ValueKindFloat ValueKind = "float"
	ValueKindBool  ValueKind = "bool"
)

type SensorProfile struct {
	Type          string    `json:"sensor_type"`
	ValueKind     ValueKind `json:"value_kind"`
	SourceChannel string    `json:"source_channel"`
	Unit          *string   `json:"unit"`
}

var BuiltInSensorProfiles = map[string]SensorProfile{
	"temperature": {
		Type:          "temperature",
		ValueKind:     ValueKindFloat,
		SourceChannel: "temperature_c",
		Unit:          strPtr("celsius"),
	},
	"humidity": {
		Type:          "humidity",
		ValueKind:     ValueKindFloat,
		SourceChannel: "humidity_pct",
		Unit:          strPtr("percent"),
	},
	"pm25": {
		Type:          "pm25",
		ValueKind:     ValueKindFloat,
		SourceChannel: "pm25_ug_m3",
		Unit:          strPtr("ug/m3"),
	},
	"pm10": {
		Type:          "pm10",
		ValueKind:     ValueKindFloat,
		SourceChannel: "pm10_ug_m3",
		Unit:          strPtr("ug/m3"),
	},
	"pressure": {
		Type:          "pressure",
		ValueKind:     ValueKindFloat,
		SourceChannel: "pressure_hpa",
		Unit:          strPtr("hPa"),
	},
	"door_open": {
		Type:          "door_open",
		ValueKind:     ValueKindBool,
		SourceChannel: "door_open",
		Unit:          nil,
	},
	"presence_detected": {
		Type:          "presence_detected",
		ValueKind:     ValueKindBool,
		SourceChannel: "presence_detected",
		Unit:          nil,
	},
}

type SensorStatus string

const (
	SensorStatusNormal   SensorStatus = "normal"
	SensorStatusDegraded SensorStatus = "degraded"
	SensorStatusOffline  SensorStatus = "offline"
	SensorStatusFaulted  SensorStatus = "faulted"
)

type Sensor struct {
	ID            string       `json:"id"`
	SiteID        string       `json:"site_id"`
	SensorType    string       `json:"sensor_type"`
	ValueKind     ValueKind    `json:"value_kind"`
	SourceChannel string       `json:"source_channel"`
	Unit          *string      `json:"unit"`
	Status        SensorStatus `json:"status"`
	Calibration   float64      `json:"calibration"`
	NoiseSigma    float64      `json:"noise_sigma"`
}

func NewSensor(id, siteID, sensorType string) (*Sensor, error) {
	profile, ok := BuiltInSensorProfiles[sensorType]
	if !ok {
		return nil, fmt.Errorf("unknown sensor type: %s", sensorType)
	}

	return &Sensor{
		ID:            id,
		SiteID:        siteID,
		SensorType:    sensorType,
		ValueKind:     profile.ValueKind,
		SourceChannel: profile.SourceChannel,
		Unit:          profile.Unit,
		Status:        SensorStatusNormal,
		Calibration:   1.0,
		NoiseSigma:    0.0,
	}, nil
}

func (s *Sensor) Validate() error {
	if s.ID == "" {
		return NewValidationError("sensor id cannot be empty")
	}
	if s.SiteID == "" {
		return NewValidationError("sensor site_id cannot be empty")
	}
	if s.ValueKind != ValueKindFloat && s.ValueKind != ValueKindBool {
		return fmt.Errorf("invalid value_kind: %s", s.ValueKind)
	}
	return nil
}

func strPtr(s string) *string {
	return &s
}
