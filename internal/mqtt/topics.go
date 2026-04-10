package mqtt

import (
	"fmt"
	"strings"
)

const topicBase = "sensimul"

// TopicLiveSensor returns the canonical sensor telemetry topic.
func TopicLiveSensor(siteID, sensorID string) string {
	return fmt.Sprintf("%s/sites/%s/sensors/%s", topicBase, siteID, sensorID)
}

// TopicLiveSensorFilter matches all normal telemetry for the web subscriber.
func TopicLiveSensorFilter() string {
	return fmt.Sprintf("%s/sites/+/sensors/+", topicBase)
}

// TopicTestRequest routes one-shot sensor test requests to the simulator.
func TopicTestRequest(siteID, sensorID string) string {
	return fmt.Sprintf("%s/tests/requests/sites/%s/sensors/%s", topicBase, siteID, sensorID)
}

// TopicTestRequestFilter matches all incoming one-shot test requests.
func TopicTestRequestFilter() string {
	return fmt.Sprintf("%s/tests/requests/sites/+/sensors/+", topicBase)
}

// TopicTestResult routes simulator one-shot test results back to web clients.
func TopicTestResult(siteID, sensorID string) string {
	return fmt.Sprintf("%s/tests/results/sites/%s/sensors/%s", topicBase, siteID, sensorID)
}

// TopicTestResultFilter matches all one-shot test results.
func TopicTestResultFilter() string {
	return fmt.Sprintf("%s/tests/results/sites/+/sensors/+", topicBase)
}

func ParseLiveTopic(topic string) (siteID, sensorID string, ok bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 {
		return "", "", false
	}
	if parts[0] != topicBase || parts[1] != "sites" || parts[3] != "sensors" {
		return "", "", false
	}
	return parts[2], parts[4], true
}

func ParseTestTopic(topic string) (kind, siteID, sensorID string, ok bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 7 {
		return "", "", "", false
	}
	if parts[0] != topicBase || parts[1] != "tests" || parts[3] != "sites" || parts[5] != "sensors" {
		return "", "", "", false
	}
	if parts[2] != "requests" && parts[2] != "results" {
		return "", "", "", false
	}
	return parts[2], parts[4], parts[6], true
}
