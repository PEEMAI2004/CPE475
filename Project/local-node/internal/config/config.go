package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all runtime configuration for the local node.
type Config struct {
	LocalMQTT        MQTTConfig  `yaml:"local_mqtt"`
	CloudMQTT        MQTTConfig  `yaml:"cloud_mqtt"`
	HTTP             HTTPConfig  `yaml:"http"`
	Store            StoreConfig `yaml:"store"`
	Database         DBConfig    `yaml:"database"`
	Prometheus       PromConfig  `yaml:"prometheus"`
	DeviceID         string      `yaml:"device_id"`
	ValidateDeviceID bool        `yaml:"validate_device_id"`
	TZOffset         int         `yaml:"tz_offset"` // Timezone offset in hours
}

// MQTTConfig holds connection settings for an MQTT broker.
type MQTTConfig struct {
	Broker   string `yaml:"broker"`
	ClientID string `yaml:"client_id"`
	SubTopic string `yaml:"sub_topic"`
	PubTopic string `yaml:"pub_topic"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	CAFile   string `yaml:"ca_file"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// HTTPConfig holds settings for the local HTTP API server.
type HTTPConfig struct {
	Port     int    `yaml:"port"`
	CAFile   string `yaml:"ca_file"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// PromConfig holds settings for Prometheus recovery.
type PromConfig struct {
	URL string `yaml:"url"`
}

// StoreConfig holds settings for the in-memory ring buffer.
type StoreConfig struct {
	BufferSize int `yaml:"buffer_size"`
}

// DBConfig holds the PostgreSQL connection string.
type DBConfig struct {
	DSN string `yaml:"dsn"`
}

// Load reads config.yaml from the given path and returns a Config.
// Environment variables override YAML values where applicable.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: decode: %w", err)
	}

	// Default to UTC+7 (Bangkok) if not set
	if cfg.TZOffset == 0 {
		cfg.TZOffset = 7
	}

	// Allow environment variable overrides.
	if v := os.Getenv("LOCAL_BROKER"); v != "" {
		cfg.LocalMQTT.Broker = v
	}
	if v := os.Getenv("LOCAL_CA_FILE"); v != "" {
		cfg.LocalMQTT.CAFile = v
	}
	if v := os.Getenv("LOCAL_CERT_FILE"); v != "" {
		cfg.LocalMQTT.CertFile = v
	}
	if v := os.Getenv("LOCAL_KEY_FILE"); v != "" {
		cfg.LocalMQTT.KeyFile = v
	}
	if v := os.Getenv("CLOUD_BROKER"); v != "" {
		cfg.CloudMQTT.Broker = v
	}
	if v := os.Getenv("CLOUD_MQTT_USER"); v != "" {
		cfg.CloudMQTT.Username = v
	}
	if v := os.Getenv("CLOUD_MQTT_PASS"); v != "" {
		cfg.CloudMQTT.Password = v
	}
	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("HTTP_CA_FILE"); v != "" {
		cfg.HTTP.CAFile = v
	}
	if v := os.Getenv("HTTP_CERT_FILE"); v != "" {
		cfg.HTTP.CertFile = v
	}
	if v := os.Getenv("HTTP_KEY_FILE"); v != "" {
		cfg.HTTP.KeyFile = v
	}
	if v := os.Getenv("PROM_URL"); v != "" {
		cfg.Prometheus.URL = v
	}
	if v := os.Getenv("DEVICE_ID"); v != "" {
		cfg.DeviceID = v
	}
	if v := os.Getenv("VALIDATE_DEVICE_ID"); v != "" {
		cfg.ValidateDeviceID = (v == "true" || v == "1")
	}
	if v := os.Getenv("TZ_OFFSET_HOURS"); v != "" {
		if offset, err := strconv.Atoi(v); err == nil {
			cfg.TZOffset = offset
		}
	}

	return &cfg, nil
}
