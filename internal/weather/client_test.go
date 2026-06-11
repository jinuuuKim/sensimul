package weather

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// The weather Client is shared across concurrent per-site simulation loops, so
// concurrent Get() must be race-free. Run with -race.
func TestClientConcurrentGet(t *testing.T) {
	c := NewClient(ModeSynthetic, "", "", "", time.Minute, time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if _, err := c.Get(); err != nil {
					t.Errorf("get: %v", err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestSyntheticModeReturnsBaseline(t *testing.T) {
	c := NewClient(ModeSynthetic, "", "", "", time.Minute, time.Second)
	w, err := c.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if w.Source != SourceSynthetic {
		t.Errorf("source = %v, want synthetic", w.Source)
	}
	if w.TemperatureC != 20.0 {
		t.Errorf("temperature = %v, want 20.0", w.TemperatureC)
	}
}

func TestKMAFetchAndCache(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Query().Get("authKey") != "secret" {
			t.Errorf("authKey not forwarded: %q", r.URL.Query().Get("authKey"))
		}
		if r.URL.Query().Get("stn") != "108" {
			t.Errorf("stn = %q, want 108", r.URL.Query().Get("stn"))
		}
		_, _ = w.Write([]byte(representativeBody))
	}))
	defer srv.Close()

	c := NewClient(ModeKMA, "secret", srv.URL, "108", time.Minute, time.Second)

	w, err := c.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if w.Source != SourceLive || w.TemperatureC != 20.9 {
		t.Fatalf("unexpected live weather: %+v", w)
	}

	// Second call within TTL must be served from cache (no extra HTTP hit).
	w2, err := c.Get()
	if err != nil {
		t.Fatalf("get cached: %v", err)
	}
	if w2.Source != SourceCache {
		t.Errorf("source = %v, want cache", w2.Source)
	}
	if hits != 1 {
		t.Errorf("http hits = %d, want 1 (TTL caching)", hits)
	}
}

func TestKMAFetchCachesByStation(t *testing.T) {
	hits := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		station := r.URL.Query().Get("stn")
		hits[station]++
		_, _ = w.Write([]byte(representativeBody))
	}))
	defer srv.Close()

	c := NewClient(ModeKMA, "secret", srv.URL, "108", time.Minute, time.Second)

	if _, err := c.GetForStation("108"); err != nil {
		t.Fatalf("get 108: %v", err)
	}
	if _, err := c.GetForStation("159"); err != nil {
		t.Fatalf("get 159: %v", err)
	}
	if _, err := c.GetForStation("108"); err != nil {
		t.Fatalf("get cached 108: %v", err)
	}

	if hits["108"] != 1 {
		t.Fatalf("station 108 hits = %d, want 1", hits["108"])
	}
	if hits["159"] != 1 {
		t.Fatalf("station 159 hits = %d, want 1", hits["159"])
	}
}

func TestKMAFailureRetainsLastGood(t *testing.T) {
	var fail bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "upstream down", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(representativeBody))
	}))
	defer srv.Close()

	now := time.Date(2025, 6, 10, 1, 0, 0, 0, time.UTC)
	c := NewClient(ModeKMA, "secret", srv.URL, "108", time.Minute, time.Second)
	c.now = func() time.Time { return now }

	if _, err := c.Get(); err != nil {
		t.Fatalf("initial get: %v", err)
	}

	// Expire the cache and make the upstream fail.
	now = now.Add(2 * time.Minute)
	fail = true

	w, err := c.Get()
	if err == nil {
		t.Fatal("expected error surfaced on upstream failure")
	}
	if w == nil {
		t.Fatal("expected last-good weather returned, got nil")
	}
	if w.TemperatureC != 20.9 {
		t.Errorf("temperature = %v, want last-good 20.9", w.TemperatureC)
	}
	if w.Source != SourceCache {
		t.Errorf("source = %v, want cache (last-good)", w.Source)
	}
}

func TestKMAEnrichesPM10WhenConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pm10" {
			_, _ = w.Write([]byte(" 202606100900 108 47"))
			return
		}
		_, _ = w.Write([]byte(representativeBody))
	}))
	defer srv.Close()

	c := NewClient(ModeKMA, "secret", srv.URL+"/sfctm", "108", time.Minute, time.Second)
	c.ConfigurePM(srv.URL+"/pm10", 2)

	w, err := c.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !w.HasPM10 || w.PM10UgM3 != 47 {
		t.Fatalf("expected PM10=47 enriched, got HasPM10=%v PM10=%v", w.HasPM10, w.PM10UgM3)
	}
	// Primary weather still intact.
	if w.TemperatureC != 20.9 {
		t.Fatalf("primary weather lost: temp=%v", w.TemperatureC)
	}
}

func TestKMAPM10FailureDoesNotFailPrimary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pm10" {
			http.Error(w, "pm down", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(representativeBody))
	}))
	defer srv.Close()

	c := NewClient(ModeKMA, "secret", srv.URL+"/sfctm", "108", time.Minute, time.Second)
	c.ConfigurePM(srv.URL+"/pm10", 2)

	w, err := c.Get()
	if err != nil {
		t.Fatalf("primary should succeed despite PM failure: %v", err)
	}
	if w.TemperatureC != 20.9 {
		t.Fatalf("primary weather lost: temp=%v", w.TemperatureC)
	}
	if w.HasPM10 {
		t.Fatalf("PM10 should be absent on PM failure with no prior, got %v", w.PM10UgM3)
	}
}

func TestKMAFailureNoPriorFallsBackToSynthetic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewClient(ModeKMA, "secret", srv.URL, "108", time.Minute, time.Second)
	w, err := c.Get()
	if err == nil {
		t.Fatal("expected error on first-fetch failure")
	}
	if w == nil || w.Source != SourceSynthetic {
		t.Fatalf("expected synthetic fallback, got %+v", w)
	}
}
