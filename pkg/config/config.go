package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DB     DBConfig     `mapstructure:"db"`
	Google GoogleConfig `mapstructure:"google"`
	Daemon DaemonConfig `mapstructure:"daemon"`
	Alerts AlertsConfig `mapstructure:"alerts"`
	Server ServerConfig `mapstructure:"server"`
	Gmail  GmailConfig  `mapstructure:"gmail"`

	Timezone string `mapstructure:"timezone"`
	LogLevel string `mapstructure:"log_level"`
}

type DBConfig struct {
	URI      string        `mapstructure:"uri"`
	Database string        `mapstructure:"database"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type GoogleConfig struct {
	CredentialsFile string `mapstructure:"credentials_file"`
	TokenFile       string `mapstructure:"token_file"`
}

type DaemonConfig struct {
	PollInterval    time.Duration `mapstructure:"poll_interval"`
	InitialLookback string        `mapstructure:"initial_lookback"`
}

type AlertsConfig struct {
	Windows map[string]int `mapstructure:"windows"`
	TTL     map[string]int `mapstructure:"ttl"`
}

type ServerConfig struct {
	Port     int    `mapstructure:"port"`
	Mode     string `mapstructure:"mode"`
	APIToken string `mapstructure:"api_token"`
}

type GmailConfig struct {
	QueryFilters    []string `mapstructure:"query_filters"`
	SenderWhitelist []string `mapstructure:"sender_whitelist"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("ASSISTANT")
	viper.AutomaticEnv()

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Location() *time.Location {
	if c == nil || c.Timezone == "" {
		loc, _ := time.LoadLocation("Asia/Kolkata")
		return loc
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		loc, _ = time.LoadLocation("UTC")
		return loc
	}
	return loc
}

func setDefaults() {
	viper.SetDefault("db.uri", "mongodb://localhost:27017")
	viper.SetDefault("db.database", "assistant-agent")
	viper.SetDefault("db.timeout", "10s")
	viper.SetDefault("google.credentials_file", "./credentials.json")
	viper.SetDefault("google.token_file", "./token.json")
	viper.SetDefault("daemon.poll_interval", "15m")
	viper.SetDefault("daemon.initial_lookback", "3m")
	viper.SetDefault("alerts.windows.birthday", 7)
	viper.SetDefault("alerts.windows.subscription", 3)
	viper.SetDefault("alerts.windows.payment", 5)
	viper.SetDefault("alerts.windows.task", 1)
	viper.SetDefault("alerts.windows.event", 2)
	viper.SetDefault("alerts.ttl.birthday", 2)
	viper.SetDefault("alerts.ttl.subscription", 7)
	viper.SetDefault("alerts.ttl.payment", 7)
	viper.SetDefault("alerts.ttl.task", 14)
	viper.SetDefault("alerts.ttl.event", 1)
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "release")
	viper.SetDefault("timezone", "Asia/Kolkata")
	viper.SetDefault("log_level", "info")
}
