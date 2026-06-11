package app

import (
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/sensimul/sensimul/internal/config"
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
)

func newTestApp(t *testing.T, siteID string) *App {
	t.Helper()
	repo, err := sqlite.New(filepath.Join(t.TempDir(), "sensimul.db"))
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return &App{
		Config: &config.Config{SiteID: siteID},
		Repo:   repo,
		Logger: zerolog.Nop(),
	}
}

func TestResolveSitesReturnsAllSites(t *testing.T) {
	a := newTestApp(t, "")
	for _, id := range []string{"SITE_A", "SITE_B", "SITE_C"} {
		if err := a.Repo.CreateSite(domain.NewSite(id, id, domain.SiteTypeIndoor, 37.5, 126.9)); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	sites, err := a.resolveSites()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(sites) != 3 {
		t.Fatalf("expected all 3 sites, got %d", len(sites))
	}
}

func TestResolveSitesHonorsConfiguredSiteID(t *testing.T) {
	a := newTestApp(t, "SITE_B")
	for _, id := range []string{"SITE_A", "SITE_B"} {
		if err := a.Repo.CreateSite(domain.NewSite(id, id, domain.SiteTypeIndoor, 37.5, 126.9)); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	sites, err := a.resolveSites()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(sites) != 1 || sites[0].ID != "SITE_B" {
		t.Fatalf("expected only SITE_B, got %+v", sites)
	}
}

func TestResolveSitesCreatesDefaultWhenEmpty(t *testing.T) {
	a := newTestApp(t, "")

	sites, err := a.resolveSites()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("expected 1 default site, got %d", len(sites))
	}
}
