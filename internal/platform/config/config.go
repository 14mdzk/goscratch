package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	App           AppConfig           `json:"app"`
	Server        ServerConfig        `json:"server"`
	Database      DatabaseConfig      `json:"database"`
	JWT           JWTConfig           `json:"jwt"`
	Redis         RedisConfig         `json:"redis"`
	RabbitMQ      RabbitMQConfig      `json:"rabbitmq"`
	Storage       StorageConfig       `json:"storage"`
	SSE           SSEConfig           `json:"sse"`
	Audit         AuditConfig         `json:"audit"`
	Authorization AuthorizationConfig `json:"authorization"`
	Worker        WorkerConfig        `json:"worker"`
	Observability ObservabilityConfig `json:"observability"`
}

type AppConfig struct {
	Name string `json:"name" env:"APP_NAME"`
	Env  string `json:"env" env:"APP_ENV"`
}

type ServerConfig struct {
	Host         string `json:"host" env:"SERVER_HOST"`
	Port         int    `json:"port" env:"SERVER_PORT"`
	ReadTimeout  int    `json:"read_timeout" env:"SERVER_READ_TIMEOUT"`
	WriteTimeout int    `json:"write_timeout" env:"SERVER_WRITE_TIMEOUT"`
	IdleTimeout  int    `json:"idle_timeout" env:"SERVER_IDLE_TIMEOUT"`
}

type DatabaseConfig struct {
	Host            string `json:"host" env:"DB_HOST"`
	Port            int    `json:"port" env:"DB_PORT"`
	User            string `json:"user" env:"DB_USER"`
	Password        string `json:"password" env:"DB_PASSWORD"`
	Name            string `json:"name" env:"DB_NAME"`
	SSLMode         string `json:"ssl_mode" env:"DB_SSL_MODE"`
	MaxOpenConns    int    `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS"`
	MaxIdleConns    int    `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime int    `json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`
}

// DSN returns the PostgreSQL connection string
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

type JWTConfig struct {
	Secret          string `json:"secret" env:"JWT_SECRET"`
	AccessTokenTTL  int    `json:"access_token_ttl" env:"JWT_ACCESS_TOKEN_TTL"`
	RefreshTokenTTL int    `json:"refresh_token_ttl" env:"JWT_REFRESH_TOKEN_TTL"`
}

// AccessTokenDuration returns the access token TTL as time.Duration (in minutes)
func (c JWTConfig) AccessTokenDuration() time.Duration {
	return time.Duration(c.AccessTokenTTL) * time.Minute
}

// RefreshTokenDuration returns the refresh token TTL as time.Duration (in minutes)
func (c JWTConfig) RefreshTokenDuration() time.Duration {
	return time.Duration(c.RefreshTokenTTL) * time.Minute
}

type RedisConfig struct {
	Enabled  bool   `json:"enabled" env:"REDIS_ENABLED"`
	Host     string `json:"host" env:"REDIS_HOST"`
	Port     int    `json:"port" env:"REDIS_PORT"`
	Password string `json:"password" env:"REDIS_PASSWORD"`
	DB       int    `json:"db" env:"REDIS_DB"`
}

// Addr returns the Redis address
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type RabbitMQConfig struct {
	Enabled bool   `json:"enabled" env:"RABBITMQ_ENABLED"`
	URL     string `json:"url" env:"RABBITMQ_URL"`
}

type StorageConfig struct {
	Mode  string       `json:"mode" env:"STORAGE_MODE"` // "local", "s3", "both"
	Local LocalStorage `json:"local"`
	S3    S3Storage    `json:"s3"`
}

type LocalStorage struct {
	BasePath string `json:"base_path" env:"STORAGE_LOCAL_PATH"`
}

type S3Storage struct {
	Endpoint  string `json:"endpoint" env:"S3_ENDPOINT"`
	Bucket    string `json:"bucket" env:"S3_BUCKET"`
	Region    string `json:"region" env:"S3_REGION"`
	AccessKey string `json:"access_key" env:"S3_ACCESS_KEY"`
	SecretKey string `json:"secret_key" env:"S3_SECRET_KEY"`
}

type SSEConfig struct {
	Enabled bool `json:"enabled" env:"SSE_ENABLED"`
}

type AuditConfig struct {
	Enabled bool `json:"enabled" env:"AUDIT_ENABLED"`
}

type AuthorizationConfig struct {
	Enabled bool `json:"enabled" env:"AUTHORIZATION_ENABLED"`
}

type WorkerConfig struct {
	Enabled     bool   `json:"enabled" env:"WORKER_ENABLED"`
	Concurrency int    `json:"concurrency" env:"WORKER_CONCURRENCY"`
	QueueName   string `json:"queue_name" env:"WORKER_QUEUE_NAME"`
	Exchange    string `json:"exchange" env:"WORKER_EXCHANGE"`
}

type ObservabilityConfig struct {
	Metrics MetricsConfig `json:"metrics"`
	Tracing TracingConfig `json:"tracing"`
}

type MetricsConfig struct {
	Enabled bool `json:"enabled" env:"METRICS_ENABLED"`
	Port    int  `json:"port" env:"METRICS_PORT"`
}

type TracingConfig struct {
	Enabled  bool   `json:"enabled" env:"TRACING_ENABLED"`
	Endpoint string `json:"endpoint" env:"TRACING_ENDPOINT"`
}

// Load reads configuration from JSON file and applies environment variable overrides
func Load(path string) (*Config, error) {
	cfg := &Config{}

	// Read JSON config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides recursively applies environment variable overrides to config struct
func applyEnvOverrides(cfg interface{}) {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			applyEnvOverrides(field.Addr().Interface())
			continue
		}

		// Get env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Get environment variable value
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		// Set field value based on type
		if !field.CanSet() {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Int, reflect.Int64:
			if intVal, err := strconv.ParseInt(envValue, 10, 64); err == nil {
				field.SetInt(intVal)
			}
		case reflect.Bool:
			field.SetBool(strings.ToLower(envValue) == "true" || envValue == "1")
		}
	}
}

// IsDevelopment returns true if the app is running in development mode
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true if the app is running in production mode
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
