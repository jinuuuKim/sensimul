package web

import (
	"testing"
	"time"
)

func TestMarkStale(t *testing.T) {
	now := time.Now().UTC()
	items := []SensorLiveReading{{SensorID: "A", Status: "normal", LastUpdated: now.Add(-20 * time.Second)}}
	out := markStale(items, 10*time.Second)
	if out[0].Status != "stale" {
		t.Fatalf("expected stale status, got %s", out[0].Status)
	}
}
