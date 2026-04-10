package mqtt

import "testing"

func TestTopicLiveRoundTrip(t *testing.T) {
	topic := TopicLiveSensor("SITE1", "TEMP1")
	siteID, sensorID, ok := ParseLiveTopic(topic)
	if !ok {
		t.Fatalf("expected live topic parse success")
	}
	if siteID != "SITE1" || sensorID != "TEMP1" {
		t.Fatalf("unexpected values: %s %s", siteID, sensorID)
	}
}

func TestTopicTestRoundTrip(t *testing.T) {
	topic := TopicTestResult("SITE2", "SEN2")
	kind, siteID, sensorID, ok := ParseTestTopic(topic)
	if !ok {
		t.Fatalf("expected test topic parse success")
	}
	if kind != "results" || siteID != "SITE2" || sensorID != "SEN2" {
		t.Fatalf("unexpected parsed topic: %s %s %s", kind, siteID, sensorID)
	}
}
