package sim

import "sync"

// FailureMode models injected simulation fault categories.
type FailureMode string

const (
	FailureModeNone          FailureMode = "none"
	FailureModeSensorStuck   FailureMode = "sensor_stuck"
	FailureModeSensorOffline FailureMode = "sensor_offline"
	FailureModePowerOutage   FailureMode = "power_outage"
)

// Failure captures the active fault state for a sensor.
type Failure struct {
	Mode     FailureMode
	SensorID string
	Value    float64
}

var activeFailures = make(map[string]*Failure)
var failureMu sync.Mutex

func InjectFailure(mode FailureMode, sensorID string, value float64) {
	failureMu.Lock()
	defer failureMu.Unlock()
	activeFailures[sensorID] = &Failure{Mode: mode, SensorID: sensorID, Value: value}
}

func ClearFailure(sensorID string) {
	failureMu.Lock()
	defer failureMu.Unlock()
	delete(activeFailures, sensorID)
}

func GetFailure(sensorID string) *Failure {
	failureMu.Lock()
	defer failureMu.Unlock()
	return activeFailures[sensorID]
}
