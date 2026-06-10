package weather

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Source string

const (
	SourceLive      Source = "live"
	SourceCache     Source = "cache"
	SourceSynthetic Source = "synthetic"
)

// Mode values.
const (
	ModeSynthetic = "synthetic"
	ModeKMA       = "kma"
)

// Weather holds the evidence values fetched from the weather source. These are
// not persisted; they only seed/adjust the simulation engines' base (ambient)
// values per the configured refresh cycle.
type Weather struct {
	TemperatureC float64
	HumidityPct  float64
	PressureHPA  float64
	WindSpeedMPS float64
	PM10UgM3     float64
	PM25UgM3     float64
	HasPM10      bool // PM10 came from the KMA 황사 source (PM2.5 is not provided)
	FetchedAt    time.Time
	Source       Source
}

type Client struct {
	Mode    string
	APIKey  string
	BaseURL string
	Station string
	TTL     time.Duration
	Timeout time.Duration

	httpClient *http.Client
	now        func() time.Time // injectable clock (tests / KST handling)

	// PM10 황사 source (opt-in). Off until ConfigurePM is called.
	pmMode    string
	pmBaseURL string
	pmColumn  int

	mu       sync.RWMutex
	stations map[string]*stationCache
}

type stationCache struct {
	weather     *Weather
	cacheTime   time.Time
	lastPM10    float64
	hasLastPM10 bool
}

// ConfigurePM enables the KMA 황사 PM10 source. baseURL is the kma_pm10.php
// endpoint and column is the 0-based PM10 column index in the data row (kept
// configurable because the exact ASOS dust layout is verified per deployment).
func (c *Client) ConfigurePM(baseURL string, column int) {
	c.pmMode = ModeKMA
	c.pmBaseURL = baseURL
	c.pmColumn = column
}

func NewClient(mode, apiKey, baseURL, station string, ttl, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		Mode:       mode,
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Station:    station,
		TTL:        ttl,
		Timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
		now:        time.Now,
		stations:   make(map[string]*stationCache),
	}
}

// Get returns the current weather evidence. It always returns a usable *Weather
// (never nil). A non-nil error means the live fetch failed but a fallback value
// (last-good cache, or synthetic) is still returned so the caller can log the
// failure while continuing the simulation without a discontinuity.
func (c *Client) Get() (*Weather, error) {
	return c.GetForStation(c.Station)
}

// GetForStation returns weather evidence for a specific KMA ASOS station. It
// keeps station-specific caches so multiple sites can use distinct weather
// sources while MQTT routing stays based on site_id.
func (c *Client) GetForStation(station string) (*Weather, error) {
	station = c.effectiveStation(station)

	// Serve a fresh cache without touching the network.
	c.mu.RLock()
	entry := c.stations[station]
	if entry != nil && entry.weather != nil && c.now().Sub(entry.cacheTime) < c.TTL {
		w := *entry.weather
		w.Source = SourceCache
		c.mu.RUnlock()
		return &w, nil
	}
	var lastGood *Weather
	if entry != nil && entry.weather != nil {
		copy := *entry.weather
		lastGood = &copy
	}
	c.mu.RUnlock()

	if c.Mode != ModeKMA {
		return c.synthetic(station), nil
	}

	// Network I/O happens WITHOUT holding the mutex.
	w, err := c.fetchKMA(station)
	if err != nil {
		// 주기적 조정 의미를 지키기 위해, 실패 시 마지막 정상값을 유지한다.
		if lastGood != nil {
			lg := *lastGood
			lg.Source = SourceCache
			return &lg, err
		}
		// 한 번도 성공한 적이 없으면 합성 기준값으로 시작한다.
		return c.synthetic(station), err
	}

	c.enrichPM(w, station)
	c.setCache(station, w)
	return w, nil
}

func (c *Client) effectiveStation(station string) string {
	if trimmed := strings.TrimSpace(station); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(c.Station)
}

// enrichPM best-effort attaches KMA 황사 PM10 to the weather. A PM failure never
// fails the primary fetch; the last-good PM10 is carried forward instead.
func (c *Client) enrichPM(w *Weather, station string) {
	if c.pmMode != ModeKMA {
		return
	}

	pm10, err := c.fetchPM10(station)
	if err != nil {
		c.mu.RLock()
		entry := c.stations[station]
		lg, ok := 0.0, false
		if entry != nil {
			lg, ok = entry.lastPM10, entry.hasLastPM10
		}
		c.mu.RUnlock()
		if ok {
			w.PM10UgM3 = lg
			w.HasPM10 = true
		}
		return
	}

	w.PM10UgM3 = pm10
	w.HasPM10 = true

	c.mu.Lock()
	entry := c.ensureStationLocked(station)
	entry.lastPM10 = pm10
	entry.hasLastPM10 = true
	c.mu.Unlock()
}

func (c *Client) synthetic(station string) *Weather {
	w := &Weather{
		TemperatureC: 20.0,
		HumidityPct:  60.0,
		PressureHPA:  1013.25,
		WindSpeedMPS: 3.5,
		FetchedAt:    c.now(),
		Source:       SourceSynthetic,
	}
	// setCache stores an independent copy, so returning w is safe.
	c.setCache(station, w)
	return w
}

func (c *Client) setCache(station string, w *Weather) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stored := *w
	entry := c.ensureStationLocked(station)
	entry.weather = &stored
	entry.cacheTime = c.now()
}

func (c *Client) ensureStationLocked(station string) *stationCache {
	entry := c.stations[station]
	if entry == nil {
		entry = &stationCache{}
		c.stations[station] = entry
	}
	return entry
}

// SetCache is retained for callers/tests that want to prime the cache directly.
func (c *Client) SetCache(w *Weather) {
	c.setCache(c.effectiveStation(c.Station), w)
}

func (c *Client) Validate() error {
	if c.Mode != ModeSynthetic && c.Mode != ModeKMA {
		return fmt.Errorf("invalid weather mode: %s (must be synthetic|kma)", c.Mode)
	}
	if c.Mode == ModeKMA {
		if c.APIKey == "" {
			return fmt.Errorf("weather api key is required for kma mode")
		}
		if c.BaseURL == "" {
			return fmt.Errorf("weather base url is required for kma mode")
		}
	}
	return nil
}
