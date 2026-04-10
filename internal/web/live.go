package web

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/sensimul/sensimul/internal/mqtt"
	"github.com/sensimul/sensimul/internal/payload"
)

// SensorPoint keeps a bounded live-only chart history point.
type SensorPoint struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

// SensorLiveReading stores the latest live telemetry snapshot for a sensor.
type SensorLiveReading struct {
	SiteID      string        `json:"site_id"`
	SensorID    string        `json:"sensor_id"`
	SensorType  string        `json:"sensor_type"`
	Value       float64       `json:"value"`
	Unit        *string       `json:"unit"`
	Status      string        `json:"status"`
	SequenceID  uint64        `json:"sequence_id"`
	Timestamp   string        `json:"timestamp"`
	LastUpdated time.Time     `json:"last_updated"`
	Points      []SensorPoint `json:"points,omitempty"`
}

type liveEvent struct {
	Kind    string                 `json:"kind"`
	Reading *SensorLiveReading     `json:"reading,omitempty"`
	Test    *mqtt.SensorTestResult `json:"test,omitempty"`
}

type LiveHub struct {
	mu       sync.RWMutex
	maxBuf   int
	readings map[string]SensorLiveReading
	history  map[string][]SensorPoint
	liveSubs map[chan []byte]struct{}
	testSubs map[string]map[chan []byte]struct{}
}

func NewLiveHub(maxBuf int) *LiveHub {
	if maxBuf < 8 {
		maxBuf = 8
	}
	return &LiveHub{
		maxBuf:   maxBuf,
		readings: make(map[string]SensorLiveReading),
		history:  make(map[string][]SensorPoint),
		liveSubs: make(map[chan []byte]struct{}),
		testSubs: make(map[string]map[chan []byte]struct{}),
	}
}

func (h *LiveHub) UpsertPayload(p *payload.Payload) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r := SensorLiveReading{
		SiteID:      p.SiteID,
		SensorID:    p.SensorID,
		SensorType:  p.SensorType,
		Value:       p.Value,
		Unit:        p.Unit,
		Status:      p.Status,
		SequenceID:  p.SequenceID,
		Timestamp:   p.Timestamp,
		LastUpdated: time.Now().UTC(),
	}

	history := append(h.history[p.SensorID], SensorPoint{At: r.LastUpdated, Value: r.Value})
	if len(history) > h.maxBuf {
		history = history[len(history)-h.maxBuf:]
	}
	h.history[p.SensorID] = history
	r.Points = append([]SensorPoint(nil), history...)
	h.readings[p.SensorID] = r

	h.broadcastLocked(liveEvent{Kind: "live", Reading: &r})
}

func (h *LiveHub) PublishTest(result mqtt.SensorTestResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	event := liveEvent{Kind: "test", Test: &result}
	body, err := json.Marshal(event)
	if err != nil {
		return
	}

	for ch := range h.liveSubs {
		select {
		case ch <- body:
		default:
		}
	}

	for ch := range h.testSubs[result.SensorID] {
		select {
		case ch <- body:
		default:
		}
	}
}

func (h *LiveHub) Readings() []SensorLiveReading {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]SensorLiveReading, 0, len(h.readings))
	for _, reading := range h.readings {
		copyReading := reading
		copyReading.Points = append([]SensorPoint(nil), h.history[reading.SensorID]...)
		out = append(out, copyReading)
	}
	return out
}

func (h *LiveHub) Reading(sensorID string) (SensorLiveReading, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	r, ok := h.readings[sensorID]
	if !ok {
		return SensorLiveReading{}, false
	}
	r.Points = append([]SensorPoint(nil), h.history[sensorID]...)
	return r, true
}

func (h *LiveHub) SubscribeLive() (chan []byte, func()) {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.liveSubs[ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		delete(h.liveSubs, ch)
		h.mu.Unlock()
		close(ch)
	}
}

func (h *LiveHub) SubscribeTest(sensorID string) (chan []byte, func()) {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	if h.testSubs[sensorID] == nil {
		h.testSubs[sensorID] = make(map[chan []byte]struct{})
	}
	h.testSubs[sensorID][ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		delete(h.testSubs[sensorID], ch)
		if len(h.testSubs[sensorID]) == 0 {
			delete(h.testSubs, sensorID)
		}
		h.mu.Unlock()
		close(ch)
	}
}

func (h *LiveHub) broadcastLocked(ev liveEvent) {
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	for ch := range h.liveSubs {
		select {
		case ch <- body:
		default:
		}
	}
}
