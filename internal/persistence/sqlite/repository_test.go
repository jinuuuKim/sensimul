package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sensimul/sensimul/internal/domain"
)

func TestRepositoryCRUD(t *testing.T) {
	repo, err := New(filepath.Join(t.TempDir(), "sensimul.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	site := domain.NewSite("S1", "Site1", domain.SiteTypeIndoor, 37.5, 126.9)
	if err := repo.CreateSite(site); err != nil {
		t.Fatalf("create site: %v", err)
	}

	site.Name = "Site1-updated"
	if err := repo.UpdateSite(site); err != nil {
		t.Fatalf("update site: %v", err)
	}

	sensor, err := domain.NewSensor("TEMP1", site.ID, "temperature")
	if err != nil {
		t.Fatalf("new sensor: %v", err)
	}
	if err := repo.CreateSensor(sensor); err != nil {
		t.Fatalf("create sensor: %v", err)
	}

	sensor.Calibration = 1.2
	if err := repo.UpdateSensor(sensor); err != nil {
		t.Fatalf("update sensor: %v", err)
	}

	ctrl, err := domain.NewController("C1", site.ID, domain.Cooling, site.Type)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	if err := repo.CreateController(ctrl); err != nil {
		t.Fatalf("create controller: %v", err)
	}

	ctrl.OutputLevel = 42
	if err := repo.UpdateController(ctrl); err != nil {
		t.Fatalf("update controller: %v", err)
	}

	if err := repo.DeleteController(ctrl.ID); err != nil {
		t.Fatalf("delete controller: %v", err)
	}
	if err := repo.DeleteSensor(sensor.ID); err != nil {
		t.Fatalf("delete sensor: %v", err)
	}
	if err := repo.DeleteSite(site.ID); err != nil {
		t.Fatalf("delete site: %v", err)
	}
}

func TestRepositoryRuntimeDurationSetting(t *testing.T) {
	repo, err := New(filepath.Join(t.TempDir(), "sensimul.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	if got, ok, err := repo.GetRuntimeDuration(RuntimeSettingTickInterval); err != nil {
		t.Fatalf("get missing setting: %v", err)
	} else if ok {
		t.Fatalf("expected missing setting, got %s", got)
	}

	if err := repo.SetRuntimeDuration(RuntimeSettingTickInterval, 750*time.Millisecond); err != nil {
		t.Fatalf("set duration: %v", err)
	}

	got, ok, err := repo.GetRuntimeDuration(RuntimeSettingTickInterval)
	if err != nil {
		t.Fatalf("get duration: %v", err)
	}
	if !ok {
		t.Fatal("expected duration setting to exist")
	}
	if got != 750*time.Millisecond {
		t.Fatalf("expected 750ms, got %s", got)
	}

	if err := repo.SetRuntimeDuration(RuntimeSettingTickInterval, 2*time.Second); err != nil {
		t.Fatalf("update duration: %v", err)
	}
	if got, _, _ := repo.GetRuntimeDuration(RuntimeSettingTickInterval); got != 2*time.Second {
		t.Fatalf("expected updated duration 2s, got %s", got)
	}
}
