package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sensimul/sensimul/internal/config"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
)

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
