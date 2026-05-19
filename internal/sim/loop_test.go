package sim

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
)

func TestLoopRuntimeTickIntervalUsesRepositorySetting(t *testing.T) {
	repo, err := sqlite.New(filepath.Join(t.TempDir(), "sensimul.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	if err := repo.SetRuntimeDuration(sqlite.RuntimeSettingTickInterval, 250*time.Millisecond); err != nil {
		t.Fatalf("set runtime interval: %v", err)
	}

	site := domain.NewSite("S1", "Site1", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	loop := NewLoop(state, LoopConfig{TickInterval: time.Second}, nil, nil, nil, repo, site.ID)

	got, err := loop.runtimeTickInterval()
	if err != nil {
		t.Fatalf("runtime interval: %v", err)
	}
	if got != 250*time.Millisecond {
		t.Fatalf("expected repository interval, got %s", got)
	}
}

func TestLoopRuntimeTickIntervalFallsBackToConfig(t *testing.T) {
	site := domain.NewSite("S1", "Site1", domain.SiteTypeIndoor, 37.5, 126.9)
	state := NewState(site, 1)
	loop := NewLoop(state, LoopConfig{TickInterval: 3 * time.Second}, nil, nil, nil, nil, site.ID)

	got, err := loop.runtimeTickInterval()
	if err != nil {
		t.Fatalf("runtime interval: %v", err)
	}
	if got != 3*time.Second {
		t.Fatalf("expected config interval, got %s", got)
	}
}
