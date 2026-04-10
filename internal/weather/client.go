package weather

import (
	"fmt"
	"sync"
	"time"
)

type Source string

const (
	SourceLive      Source = "live"
	SourceCache     Source = "cache"
	SourceSynthetic Source = "synthetic"
)

type Weather struct {
	TemperatureC float64
	HumidityPct  float64
	PressureHPA  float64
	WindSpeedMPS float64
	FetchedAt    time.Time
	Source       Source
}

type Client struct {
	Mode      string
	APIKey    string
	TTL       time.Duration
	Cache     *Weather
	CacheTime time.Time
	mu        sync.RWMutex
}

func NewClient(mode, apiKey string, ttl time.Duration) *Client {
	return &Client{
		Mode:   mode,
		APIKey: apiKey,
		TTL:    ttl,
	}
}

func (c *Client) Get() (*Weather, error) {
	c.mu.RLock()
	if c.Cache != nil && time.Since(c.CacheTime) < c.TTL {
		w := c.Cache
		w.Source = SourceCache
		c.mu.RUnlock()
		return w, nil
	}
	c.mu.RUnlock()

	if c.Mode == "synthetic" {
		return c.synthetic(), nil
	}

	return c.synthetic(), nil
}

func (c *Client) synthetic() *Weather {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := &Weather{
		TemperatureC: 20.0,
		HumidityPct:  60.0,
		PressureHPA:  1013.25,
		WindSpeedMPS: 3.5,
		FetchedAt:    time.Now(),
		Source:       SourceSynthetic,
	}
	c.Cache = w
	c.CacheTime = time.Now()
	return w
}

func (c *Client) SetCache(w *Weather) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Cache = w
	c.CacheTime = time.Now()
}

func (c *Client) Validate() error {
	if c.Mode != "synthetic" && c.Mode != "openweathermap" {
		return fmt.Errorf("invalid weather mode: %s", c.Mode)
	}
	return nil
}
