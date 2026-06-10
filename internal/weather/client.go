package weather

import (
	"fmt"
	"net/http"
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

	mu        sync.RWMutex
	cache     *Weather
	cacheTime time.Time
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
	}
}

// Get returns the current weather evidence. It always returns a usable *Weather
// (never nil). A non-nil error means the live fetch failed but a fallback value
// (last-good cache, or synthetic) is still returned so the caller can log the
// failure while continuing the simulation without a discontinuity.
func (c *Client) Get() (*Weather, error) {
	// Serve a fresh cache without touching the network.
	c.mu.RLock()
	if c.cache != nil && c.now().Sub(c.cacheTime) < c.TTL {
		w := *c.cache
		w.Source = SourceCache
		c.mu.RUnlock()
		return &w, nil
	}
	lastGood := c.cache // may be nil; copy pointer under lock
	c.mu.RUnlock()

	if c.Mode != ModeKMA {
		return c.synthetic(), nil
	}

	// Network I/O happens WITHOUT holding the mutex.
	w, err := c.fetchKMA()
	if err != nil {
		// 주기적 조정 의미를 지키기 위해, 실패 시 마지막 정상값을 유지한다.
		if lastGood != nil {
			lg := *lastGood
			lg.Source = SourceCache
			return &lg, err
		}
		// 한 번도 성공한 적이 없으면 합성 기준값으로 시작한다.
		return c.synthetic(), err
	}

	c.setCache(w)
	return w, nil
}

func (c *Client) synthetic() *Weather {
	w := &Weather{
		TemperatureC: 20.0,
		HumidityPct:  60.0,
		PressureHPA:  1013.25,
		WindSpeedMPS: 3.5,
		FetchedAt:    c.now(),
		Source:       SourceSynthetic,
	}
	// setCache stores an independent copy, so returning w is safe.
	c.setCache(w)
	return w
}

func (c *Client) setCache(w *Weather) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stored := *w
	c.cache = &stored
	c.cacheTime = c.now()
}

// SetCache is retained for callers/tests that want to prime the cache directly.
func (c *Client) SetCache(w *Weather) {
	c.setCache(w)
}

func (c *Client) Validate() error {
	if c.Mode != ModeSynthetic && c.Mode != ModeKMA {
		return fmt.Errorf("invalid weather mode: %s (must be synthetic|kma)", c.Mode)
	}
	if c.Mode == ModeKMA {
		if c.APIKey == "" {
			return fmt.Errorf("weather api key is required for kma mode")
		}
		if c.Station == "" {
			return fmt.Errorf("weather station is required for kma mode")
		}
		if c.BaseURL == "" {
			return fmt.Errorf("weather base url is required for kma mode")
		}
	}
	return nil
}
