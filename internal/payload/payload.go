package payload

import (
	"encoding/json"
	"fmt"
	"time"
)

type Payload struct {
	SiteID        string  `json:"site_id"`
	SensorID      string  `json:"sensor_id"`
	SensorType    string  `json:"sensor_type"`
	ValueKind     string  `json:"value_kind"`
	Value         float64 `json:"value"`
	Unit          *string `json:"unit"`
	Status        string  `json:"status"`
	Timestamp     string  `json:"timestamp"`
	SequenceID    uint64  `json:"sequence_id"`
	SchemaVersion string  `json:"schema_version"`
}

func New(siteID, sensorID, sensorType, valueKind string, value float64, unit *string, status string, seq uint64) *Payload {
	return &Payload{
		SiteID:        siteID,
		SensorID:      sensorID,
		SensorType:    sensorType,
		ValueKind:     valueKind,
		Value:         value,
		Unit:          unit,
		Status:        status,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		SequenceID:    seq,
		SchemaVersion: "1.0",
	}
}

func (p *Payload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Payload) ToJSONString() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func FromJSON(data []byte) (*Payload, error) {
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return &p, nil
}
