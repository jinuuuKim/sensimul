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

func TestTopicControllerCommandRoundTrip(t *testing.T) {
	topic := TopicControllerCommand("SITE1", "CTRL1")
	if topic != "sensimul/sites/SITE1/controllers/CTRL1/commands" {
		t.Fatalf("unexpected command topic: %s", topic)
	}
	siteID, controllerID, ok := ParseControllerCommandTopic(topic)
	if !ok || siteID != "SITE1" || controllerID != "CTRL1" {
		t.Fatalf("expected command topic parse, got site=%s ctrl=%s ok=%v", siteID, controllerID, ok)
	}
	if _, _, ok := ParseControllerCommandTopic("sensimul/sites/S/controllers/C/acks"); ok {
		t.Fatalf("ack topic must not parse as command topic")
	}
	if ack := TopicControllerAck("SITE1", "CTRL1"); ack != "sensimul/sites/SITE1/controllers/CTRL1/acks" {
		t.Fatalf("unexpected ack topic: %s", ack)
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
