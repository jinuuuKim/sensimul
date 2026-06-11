package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sensimul/sensimul/internal/config"
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
)

func TestTemplatesParseAndRender(t *testing.T) {
	tpls, err := parseTemplates()
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	unit := "°C"
	reading := SensorLiveReading{
		SiteID: "S1", SensorID: "TEMP_1", SensorType: "temperature",
		Value: 21.5, Unit: &unit, Status: "normal", LastUpdated: time.Now().UTC(),
		Points: []SensorPoint{{At: time.Now().UTC(), Value: 21.4}, {At: time.Now().UTC(), Value: 21.5}},
	}

	cases := []struct {
		name string
		data viewData
	}{
		{"live", viewData{Title: "Live", Live: []SensorLiveReading{reading}, StaleAfter: 10 * time.Second, RuntimeTickInterval: "5s"}},
		{"live", viewData{Title: "Live"}}, // empty -> placeholder branch
		{"live-sensor", viewData{Title: "Sensor", Sensor: &domain.Sensor{ID: "TEMP_1", SiteID: "S1", SensorType: "temperature"}, LiveOne: &reading, StaleAfter: 10 * time.Second}},
	}

	for _, c := range cases {
		var buf bytes.Buffer
		if err := tpls.ExecuteTemplate(&buf, c.name, c.data); err != nil {
			t.Fatalf("render %q: %v", c.name, err)
		}
		if buf.Len() == 0 {
			t.Fatalf("render %q produced empty output", c.name)
		}
	}
}

func TestMarkStale(t *testing.T) {
	now := time.Now().UTC()
	items := []SensorLiveReading{{SensorID: "A", Status: "normal", LastUpdated: now.Add(-20 * time.Second)}}
	out := markStale(items, 10*time.Second)
	if out[0].Status != "stale" {
		t.Fatalf("expected stale status, got %s", out[0].Status)
	}
}

func TestServerRuntimeTickIntervalUsesRepositorySetting(t *testing.T) {
	repo, err := sqlite.New(filepath.Join(t.TempDir(), "sensimul.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	if err := repo.SetRuntimeDuration(sqlite.RuntimeSettingTickInterval, 750*time.Millisecond); err != nil {
		t.Fatalf("set interval: %v", err)
	}

	s := &Server{cfg: &config.Config{TickInterval: time.Second}, repo: repo}
	got, err := s.runtimeTickInterval()
	if err != nil {
		t.Fatalf("runtime interval: %v", err)
	}
	if got != 750*time.Millisecond {
		t.Fatalf("expected repository interval, got %s", got)
	}
}

func TestHandleTickIntervalUpdateStoresRuntimeSetting(t *testing.T) {
	repo, err := sqlite.New(filepath.Join(t.TempDir(), "sensimul.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	s := &Server{
		cfg:        &config.Config{TickInterval: time.Second},
		repo:       repo,
		hub:        NewLiveHub(8),
		staleAfter: 10 * time.Second,
	}

	form := url.Values{"tick_interval": {"250ms"}}
	req := httptest.NewRequest(http.MethodPost, "/settings/tick-interval", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.handleTickIntervalUpdate(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", w.Code)
	}
	got, ok, err := repo.GetRuntimeDuration(sqlite.RuntimeSettingTickInterval)
	if err != nil {
		t.Fatalf("get interval: %v", err)
	}
	if !ok {
		t.Fatal("expected interval setting to exist")
	}
	if got != 250*time.Millisecond {
		t.Fatalf("expected stored interval 250ms, got %s", got)
	}
}
