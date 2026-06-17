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
	ID         string           `json:"id"`
	SiteID     string           `json:"site_id"`
	Type       ControllerType   `json:"type"`
	TargetAxis TargetAxis       `json:"target_axis"`
	Status     ControllerStatus `json:"status"`
	// OutputLevel is the device output (0-100). In target mode it is computed by
	// the simulator each tick (feed-forward from the weather ambient) and persisted
	// for display; in legacy/manual mode it is set directly by the operator.
	OutputLevel int `json:"output_level"`
	// TargetValue is the operator's desired value on the controller's axis
	// (temperature °C / humidity %). Honored only when HasTarget is true.
	TargetValue float64 `json:"target_value"`
	HasTarget   bool    `json:"has_target"`
}

// RequiredOutput computes the feed-forward output level (0-100) needed to hold
// TargetValue given the current ambient (weather) value, by inverting the engine
// equilibrium. Returns 0 when the controller is not needed in its own direction
// (e.g. a cooler when the room is already at/below its target). When no target is
// set it falls back to the manually configured OutputLevel.
//
// Equilibrium of the physics engines (see internal/sim/physics) with the
// resolveControllers power mapping power = output/20:
//
//	cooling:       Eq = ambient - output/2   => output = 2*(ambient - target)
//	heating:       Eq = ambient + output/2   => output = 2*(target - ambient)
//	humidifying:   Eq = ambient + output     => output =   (target - ambient)
//	dehumidifying: Eq = ambient - output     => output =   (ambient - target)
//
// The loop integration tests pin these to the real engines, so a physics-constant
// change that breaks the inversion is caught there.
func (c *Controller) RequiredOutput(ambient float64) int {
	if !c.HasTarget {
		return c.OutputLevel
	}

	var raw float64
	switch c.Type {
	case Cooling:
		raw = 2 * (ambient - c.TargetValue)
	case Heating:
		raw = 2 * (c.TargetValue - ambient)
	case Humidifying:
		raw = c.TargetValue - ambient
	case Dehumidifying:
		raw = ambient - c.TargetValue
	default:
		// Air-quality controllers (ventilation/air_purifier) are not setpoint-driven.
		return 0
	}

	if raw <= 0 {
		return 0
	}
	if raw > 100 {
		return 100
	}
	return int(raw + 0.5)
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
