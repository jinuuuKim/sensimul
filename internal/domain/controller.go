package domain

import (
	"fmt"
)

type ControllerType string

const (
	Cooling       ControllerType = "cooling"
	Heating       ControllerType = "heating"
	Humidifying   ControllerType = "humidifying"
	Dehumidifying ControllerType = "dehumidifying"
	Ventilation   ControllerType = "ventilation"
	AirPurifier   ControllerType = "air_purifier"
)

type TargetAxis string

const (
	AxisTemperature TargetAxis = "temperature"
	AxisHumidity    TargetAxis = "humidity"
	AxisAirQuality  TargetAxis = "air_quality"
)

var ControllerTypeToAxis = map[ControllerType]TargetAxis{
	Cooling:       AxisTemperature,
	Heating:       AxisTemperature,
	Humidifying:   AxisHumidity,
	Dehumidifying: AxisHumidity,
	Ventilation:   AxisAirQuality,
	AirPurifier:   AxisAirQuality,
}

var ConflictGroups = [][]ControllerType{
	{Cooling, Heating},
	{Humidifying, Dehumidifying},
}

type ControllerStatus string

const (
	ControllerStatusOn  ControllerStatus = "on"
	ControllerStatusOff ControllerStatus = "off"
)

type Controller struct {
	ID          string           `json:"id"`
	SiteID      string           `json:"site_id"`
	Type        ControllerType   `json:"type"`
	TargetAxis  TargetAxis       `json:"target_axis"`
	Status      ControllerStatus `json:"status"`
	OutputLevel int              `json:"output_level"`
}

func NewController(id, siteID string, ctrlType ControllerType, siteType SiteType) (*Controller, error) {
	if siteType != SiteTypeIndoor {
		return nil, NewValidationError("controllers are only supported for indoor sites")
	}

	axis, ok := ControllerTypeToAxis[ctrlType]
	if !ok {
		return nil, fmt.Errorf("unknown controller type: %s", ctrlType)
	}

	return &Controller{
		ID:          id,
		SiteID:      siteID,
		Type:        ctrlType,
		TargetAxis:  axis,
		Status:      ControllerStatusOff,
		OutputLevel: 0,
	}, nil
}

func (c *Controller) Validate() error {
	if c.ID == "" {
		return NewValidationError("controller id cannot be empty")
	}
	if c.SiteID == "" {
		return NewValidationError("controller site_id cannot be empty")
	}
	if c.OutputLevel < 0 || c.OutputLevel > 100 {
		return fmt.Errorf("output_level must be between 0 and 100, got %d", c.OutputLevel)
	}
	return nil
}

func AreControllersConflicting(a, b *Controller) bool {
	for _, group := range ConflictGroups {
		if containsController(group, a.Type) && containsController(group, b.Type) {
			return true
		}
	}
	return false
}

func containsController(group []ControllerType, t ControllerType) bool {
	for _, g := range group {
		if g == t {
			return true
		}
	}
	return false
}
