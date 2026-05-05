package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validConfigJSON = `{
	"app": {
		"name": "goscratch",
		"env": "development"
	},
	"server": {
		"host": "0.0.0.0",
		"port": 3000,
		"read_timeout": 10,
		"write_timeout": 10,
		"idle_timeout": 30
	},
	"database": {
		"host": "localhost",
		"port": 5432,
		"user": "postgres",
		"password": "secret",
		"name": "goscratch",
		"ssl_mode": "disable",
		"max_open_conns": 25,
		"max_idle_conns": 5,
		"conn_max_lifetime": 300
	},
	"jwt": {
		"secret": "my-jwt-secret",
		"access_token_ttl": 15,
		"refresh_token_ttl": 10080
	},
	"redis": {
		"enabled": true,
		"host": "localhost",
		"port": 6379,
		"password": "",
		"db": 0
	},
	"rabbitmq": {
		"enabled": false,
		"url": "amqp://guest:guest@localhost:5672/"
	},
	"storage": {
		"mode": "local",
		"local": {"base_path": "/tmp/uploads"},
		"s3": {
			"endpoint": "",
			"bucket": "",
			"region": "",
			"access_key": "",
			"secret_key": ""
		}
	},
	"sse": {"enabled": false},
	"audit": {"enabled": false},
	"authorization": {"enabled": false},
	"worker": {
		"enabled": false,
		"concurrency": 5,
		"queue_name": "tasks",
		"exchange": "goscratch"
	},
	"observability": {
		"metrics": {"enabled": false, "port": 9090},
		"tracing": {"enabled": false, "endpoint": ""}
	}
}`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func TestLoad_ValidJSON(t *testing.T) {
	path := writeTempConfig(t, validConfigJSON)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "goscratch", cfg.App.Name)
	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "postgres", cfg.Database.User)
	assert.Equal(t, "secret", cfg.Database.Password)
	assert.Equal(t, "goscratch", cfg.Database.Name)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, "my-jwt-secret", cfg.JWT.Secret)
	assert.Equal(t, 15, cfg.JWT.AccessTokenTTL)
	assert.Equal(t, 10080, cfg.JWT.RefreshTokenTTL)
	assert.True(t, cfg.Redis.Enabled)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.False(t, cfg.RabbitMQ.Enabled)
	assert.Equal(t, 5, cfg.Worker.Concurrency)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := writeTempConfig(t, `{invalid json}`)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestApplyEnvOverrides_String(t *testing.T) {
	path := writeTempConfig(t, validConfigJSON)
	t.Setenv("APP_NAME", "overridden-name")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "overridden-name", cfg.App.Name)
}

func TestApplyEnvOverrides_Int(t *testing.T) {
	path := writeTempConfig(t, validConfigJSON)
	t.Setenv("SERVER_PORT", "8080")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestApplyEnvOverrides_Bool(t *testing.T) {
	path := writeTempConfig(t, validConfigJSON)
	t.Setenv("REDIS_ENABLED", "false")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.False(t, cfg.Redis.Enabled)
}

func TestApplyEnvOverrides_BoolNumeric(t *testing.T) {
	path := writeTempConfig(t, validConfigJSON)
	// JSON has enabled: false for rabbitmq, override with "1"
	t.Setenv("RABBITMQ_ENABLED", "1")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.RabbitMQ.Enabled)
}

func TestEnvVarsTakePrecedence(t *testing.T) {
	path := writeTempConfig(t, validConfigJSON)
	t.Setenv("DB_HOST", "remotehost")
	t.Setenv("DB_PORT", "9999")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "remotehost", cfg.Database.Host)
	assert.Equal(t, 9999, cfg.Database.Port)
}

func TestIsDevelopment(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "development"}}
	assert.True(t, cfg.IsDevelopment())
	assert.False(t, cfg.IsProduction())
}

func TestIsProduction(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "production"}}
	assert.True(t, cfg.IsProduction())
	assert.False(t, cfg.IsDevelopment())
}

func TestDSN(t *testing.T) {
	db := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Name:     "mydb",
		SSLMode:  "disable",
	}
	expected := "postgres://postgres:secret@localhost:5432/mydb?sslmode=disable"
	assert.Equal(t, expected, db.DSN())
}

func TestRedisAddr(t *testing.T) {
	r := RedisConfig{Host: "localhost", Port: 6379}
	assert.Equal(t, "localhost:6379", r.Addr())
}

func TestValidate_RejectsPlaceholderJWTSecret(t *testing.T) {
	cfg := &Config{JWT: JWTConfig{Secret: PlaceholderJWTSecret}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
	assert.Contains(t, err.Error(), "placeholder")
}

func TestValidate_RejectsShortJWTSecret(t *testing.T) {
	cfg := &Config{JWT: JWTConfig{Secret: "too-short"}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidate_RejectsEmptyJWTSecret(t *testing.T) {
	cfg := &Config{JWT: JWTConfig{Secret: ""}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidate_AcceptsLongUniqueJWTSecret(t *testing.T) {
	// 32 bytes, not the placeholder, with non-empty issuer and audience.
	cfg := &Config{JWT: JWTConfig{
		Secret:   "a-32-byte-real-secret-xxxxxxxxxx",
		Issuer:   "goscratch",
		Audience: "goscratch-api",
	}}
	require.Len(t, cfg.JWT.Secret, MinJWTSecretLen)
	require.NoError(t, cfg.Validate())
}

func TestValidate_RejectsEmptyIssuer(t *testing.T) {
	cfg := &Config{JWT: JWTConfig{
		Secret:   "a-32-byte-real-secret-xxxxxxxxxx",
		Issuer:   "",
		Audience: "goscratch-api",
	}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jwt.issuer")
}

func TestValidate_RejectsEmptyAudience(t *testing.T) {
	cfg := &Config{JWT: JWTConfig{
		Secret:   "a-32-byte-real-secret-xxxxxxxxxx",
		Issuer:   "goscratch",
		Audience: "",
	}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jwt.audience")
}

func TestJWTDurations(t *testing.T) {
	j := JWTConfig{
		AccessTokenTTL:  15,
		RefreshTokenTTL: 10080,
	}
	assert.Equal(t, "15m0s", j.AccessTokenDuration().String())
	assert.Equal(t, "168h0m0s", j.RefreshTokenDuration().String())
}
