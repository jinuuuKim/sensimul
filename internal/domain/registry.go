package domain

type Registry struct {
	sensors     map[string]SensorProfile
	controllers map[ControllerType]TargetAxis
}

func NewRegistry() *Registry {
	r := &Registry{
		sensors:     make(map[string]SensorProfile),
		controllers: make(map[ControllerType]TargetAxis),
	}
	for k, v := range BuiltInSensorProfiles {
		r.sensors[k] = v
	}
	for k, v := range ControllerTypeToAxis {
		r.controllers[k] = v
	}
	return r
}

func (r *Registry) GetSensorProfile(sensorType string) (SensorProfile, bool) {
	p, ok := r.sensors[sensorType]
	return p, ok
}

func (r *Registry) RegisterSensorProfile(profile SensorProfile) error {
	if profile.Type == "" {
		return NewValidationError("sensor type cannot be empty")
	}
	if profile.ValueKind != ValueKindFloat && profile.ValueKind != ValueKindBool {
		return NewValidationError("value_kind must be float or bool")
	}
	r.sensors[profile.Type] = profile
	return nil
}

func (r *Registry) ListSensorTypes() []string {
	types := make([]string, 0, len(r.sensors))
	for t := range r.sensors {
		types = append(types, t)
	}
	return types
}

func (r *Registry) GetControllerAxis(ctrlType ControllerType) (TargetAxis, bool) {
	axis, ok := r.controllers[ctrlType]
	return axis, ok
}
