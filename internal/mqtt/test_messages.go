package mqtt

import "time"

// SensorTestRequest is emitted by web UI for one-shot test execution.
type SensorTestRequest struct {
	SiteID    string `json:"site_id"`
	SensorID  string `json:"sensor_id"`
	Requested string `json:"requested_at"`
}

// SensorTestResult is emitted by simulator once per request.
type SensorTestResult struct {
	SiteID      string  `json:"site_id"`
	SensorID    string  `json:"sensor_id"`
	SensorType  string  `json:"sensor_type"`
	Value       float64 `json:"value"`
	Unit        *string `json:"unit"`
	Status      string  `json:"status"`
	SequenceID  uint64  `json:"sequence_id"`
	RespondedAt string  `json:"responded_at"`
}

// ControllerCommand is published by the API server to drive a controller.
type ControllerCommand struct {
	CorrelationID string `json:"correlationId"`
	SiteID        string `json:"siteId"`
	ControllerID  string `json:"controllerId"`
	Status        string `json:"status"`
	OutputLevel   *int   `json:"outputLevel"`
}

// ControllerCommandAck is published by the simulator after applying (or rejecting) a command.
type ControllerCommandAck struct {
	CorrelationID string `json:"correlationId"`
	ResultStatus  string `json:"resultStatus"`
	ResultCode    string `json:"resultCode"`
	Message       string `json:"message"`
}

func NewSensorTestRequest(siteID, sensorID string) SensorTestRequest {
	return SensorTestRequest{
		SiteID:    siteID,
		SensorID:  sensorID,
		Requested: time.Now().UTC().Format(time.RFC3339),
	}
}

func NewSensorTestResult(siteID, sensorID, sensorType string, value float64, unit *string, status string, seq uint64) SensorTestResult {
	return SensorTestResult{
		SiteID:      siteID,
		SensorID:    sensorID,
		SensorType:  sensorType,
		Value:       value,
		Unit:        unit,
		Status:      status,
		SequenceID:  seq,
		RespondedAt: time.Now().UTC().Format(time.RFC3339),
	}
}
