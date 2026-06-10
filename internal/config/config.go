package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Mode         string        `mapstructure:"mode"`
	Seed         int64         `mapstructure:"seed"`
	TickInterval time.Duration `mapstructure:"tick_interval"`
	SiteID       string        `mapstructure:"site_id"`
	SQLite       SQLiteConfig  `mapstructure:"sqlite"`
	MQTT         MQTTConfig    `mapstructure:"mqtt"`
	Weather      WeatherConfig `mapstructure:"weather"`
	Logging      LoggingConfig `mapstructure:"logging"`
	Web          WebConfig     `mapstructure:"web"`
}

type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

type MQTTConfig struct {
	BrokerURL string `mapstructure:"broker_url"`
	ClientID  string `mapstructure:"client_id"`
	QoS       byte   `mapstructure:"qos"`
	Retain    bool   `mapstructure:"retain"`
}

type WeatherConfig struct {
	Mode    string        `mapstructure:"mode"`     // synthetic | kma
	APIKey  string        `mapstructure:"api_key"`  // KMA API Hub authKey (mode=kma)
	BaseURL string        `mapstructure:"base_url"` // KMA ASOS hourly endpoint
	Station string        `mapstructure:"station"`  // ASOS station number (지점번호), default 108(서울)
	TTL     time.Duration `mapstructure:"ttl"`      // refresh/cache cycle; KMA ASOS is hourly
	Timeout time.Duration `mapstructure:"timeout"`  // HTTP request timeout (mode=kma)

	// PM10 황사 source (opt-in). off | kma. PM2.5 is not provided by KMA 황사
	// and stays simulated. Off by default; enable after verifying the response
	// layout against a live kma_pm10.php call.
	PMMode    string `mapstructure:"pm_mode"`     // off | kma
	PMBaseURL string `mapstructure:"pm_base_url"` // KMA 황사 PM10 endpoint
	PMColumn  int    `mapstructure:"pm_column"`   // 0-based PM10 column index in the data row
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// WebConfig controls the standalone web client service runtime.
type WebConfig struct {
	ListenAddr string        `mapstructure:"listen_addr"`
	StaleAfter time.Duration `mapstructure:"stale_after"`
	SSEBuffer  int           `mapstructure:"sse_buffer"`
}

var defaultConfig = Config{
	Mode:         "dev",
	Seed:         1,
	TickInterval: 5 * time.Second,
	SQLite: SQLiteConfig{
		Path: "data/sensimul.db",
	},
	MQTT: MQTTConfig{
		BrokerURL: "tcp://localhost:1883",
		QoS:       1,
		Retain:    false,
	},
	Weather: WeatherConfig{
		Mode:      "synthetic",
		BaseURL:   "https://apihub.kma.go.kr/api/typ01/url/kma_sfctm2.php",
		Station:   "108",
		TTL:       3600 * time.Second, // KMA ASOS 시간자료는 매시간 갱신 → 1시간 주기
		Timeout:   10 * time.Second,
		PMMode:    "off",
		PMBaseURL: "https://apihub.kma.go.kr/api/typ01/url/kma_pm10.php",
		PMColumn:  2,
	},
	Logging: LoggingConfig{
		Level:  "info",
		Format: "json",
	},
	Web: WebConfig{
		ListenAddr: ":8080",
		StaleAfter: 10 * time.Second,
		SSEBuffer:  256,
	},
}

var globalConfig Config

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	v.SetEnvPrefix("SENSIMUL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	config := defaultConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	globalConfig = config
	return &config, nil
}

func validate(c *Config) error {
	if c.Mode != "dev" && c.Mode != "test" && c.Mode != "prodlike" {
		return fmt.Errorf("invalid mode: %s (must be dev|test|prodlike)", c.Mode)
	}
	if c.Seed < 0 {
		return fmt.Errorf("seed must be non-negative")
	}
	if c.TickInterval <= 0 {
		return fmt.Errorf("tick_interval must be positive")
	}
	if c.Weather.Mode != "synthetic" && c.Weather.Mode != "kma" {
		return fmt.Errorf("invalid weather mode: %s (must be synthetic|kma)", c.Weather.Mode)
	}
	if c.Weather.Mode == "kma" {
		if c.Weather.APIKey == "" {
			return fmt.Errorf("weather.api_key is required when weather.mode is kma")
		}
		if c.Weather.Station == "" {
			return fmt.Errorf("weather.station is required when weather.mode is kma")
		}
		if c.Weather.BaseURL == "" {
			return fmt.Errorf("weather.base_url is required when weather.mode is kma")
		}
		if c.Weather.Timeout <= 0 {
			return fmt.Errorf("weather.timeout must be positive when weather.mode is kma")
		}
	}
	if c.Weather.PMMode != "off" && c.Weather.PMMode != "kma" {
		return fmt.Errorf("invalid weather.pm_mode: %s (must be off|kma)", c.Weather.PMMode)
	}
	if c.Weather.PMMode == "kma" {
		if c.Weather.Mode != "kma" {
			return fmt.Errorf("weather.pm_mode=kma requires weather.mode=kma (shared authKey)")
		}
		if c.Weather.PMBaseURL == "" {
			return fmt.Errorf("weather.pm_base_url is required when weather.pm_mode is kma")
		}
		if c.Weather.PMColumn < 0 {
			return fmt.Errorf("weather.pm_column must be >= 0")
		}
	}
	if c.MQTT.QoS > 2 {
		return fmt.Errorf("mqtt qos must be 0|1|2")
	}
	if c.Web.ListenAddr == "" {
		return fmt.Errorf("web.listen_addr cannot be empty")
	}
	if c.Web.StaleAfter <= 0 {
		return fmt.Errorf("web.stale_after must be positive")
	}
	if c.Web.SSEBuffer < 1 {
		return fmt.Errorf("web.sse_buffer must be at least 1")
	}
	return nil
}

func Get() *Config {
	return &globalConfig
}

func MustLoad(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load error: %v\n", err)
		os.Exit(2)
	}
	return cfg
}
