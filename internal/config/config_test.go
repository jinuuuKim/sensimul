package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Verifies the deploy-time mechanism: the KMA authKey is injected via the
// SENSIMUL_WEATHER_API_KEY environment variable and never stored in the YAML.
// validate() requires a non-empty api_key for mode=kma, so if the env override
// did not apply, Load would fail — making this a strict check of the binding.
func TestEnvOverridesWeatherAPIKey(t *testing.T) {
	t.Setenv("SENSIMUL_WEATHER_API_KEY", "from-env-secret")

	dir := t.TempDir()
	path := filepath.Join(dir, "sensimul.yaml")
	body := `mode: dev
weather:
  mode: kma
  api_key: ""
  base_url: https://example.com/sfctm2.php
  station: "108"
  ttl: 3600s
  timeout: 10s
web:
  listen_addr: ":8080"
  stale_after: 10s
  sse_buffer: 256
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Weather.APIKey != "from-env-secret" {
		t.Fatalf("api_key = %q, want env override 'from-env-secret'", cfg.Weather.APIKey)
	}
}
