package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// 基础配置
	MaxConcurrentHandlers int           `json:"max_concurrent_handlers" yaml:"max_concurrent_handlers" env:"BOT_MAX_CONCURRENT_HANDLERS" default:"100"`
	HandlerTimeout        time.Duration `json:"handler_timeout" yaml:"handler_timeout" env:"BOT_HANDLER_TIMEOUT" default:"30s"`
	EnableMetrics         bool          `json:"enable_metrics" yaml:"enable_metrics" env:"BOT_ENABLE_METRICS" default:"true"`
	LogLevel              string        `json:"log_level" yaml:"log_level" env:"BOT_LOG_LEVEL" default:"info"`

	// 连接配置
	Connection ConnectionConfig `json:"connection" yaml:"connection"`

	// 重试配置
	Retry RetryConfig `json:"retry" yaml:"retry"`

	// 插件配置
	Plugins map[string]interface{} `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

type ConnectionConfig struct {
	// WebSocket 配置
	WebSocket WebSocketConnectionConfig `json:"websocket" yaml:"websocket"`

	// HTTP 配置
	HTTP HTTPConnectionConfig `json:"http" yaml:"http"`

	// 超时配置
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout" env:"BOT_CONNECT_TIMEOUT" default:"10s"`
	ReadTimeout    time.Duration `json:"read_timeout" yaml:"read_timeout" env:"BOT_READ_TIMEOUT" default:"60s"`
	WriteTimeout   time.Duration `json:"write_timeout" yaml:"write_timeout" env:"BOT_WRITE_TIMEOUT" default:"10s"`

	// 心跳配置
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval" env:"BOT_HEARTBEAT_INTERVAL" default:"30s"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout" yaml:"heartbeat_timeout" env:"BOT_HEARTBEAT_TIMEOUT" default:"5s"`

	// 重连配置
	AutoReconnect     bool          `json:"auto_reconnect" yaml:"auto_reconnect" env:"BOT_AUTO_RECONNECT" default:"true"`
	ReconnectDelay    time.Duration `json:"reconnect_delay" yaml:"reconnect_delay" env:"BOT_RECONNECT_DELAY" default:"5s"`
	MaxReconnectTries int           `json:"max_reconnect_tries" yaml:"max_reconnect_tries" env:"BOT_MAX_RECONNECT_TRIES" default:"10"`
}

type WebSocketConnectionConfig struct {
	URL         string `json:"url" yaml:"url" env:"BOT_WS_URL"`
	AccessToken string `json:"access_token" yaml:"access_token" env:"BOT_ACCESS_TOKEN"`
}

type HTTPConnectionConfig struct {
	Host        string `json:"host" yaml:"host" env:"BOT_HTTP_HOST" default:"localhost"`
	Port        int    `json:"port" yaml:"port" env:"BOT_HTTP_PORT" default:"5700"`
	AccessToken string `json:"access_token" yaml:"access_token" env:"BOT_HTTP_ACCESS_TOKEN"`
	Secret      string `json:"secret" yaml:"secret" env:"BOT_HTTP_SECRET"`
}

// ConfigLoader 配置加载器
type ConfigLoader struct {
	configPaths []string
}

func NewConfigLoader(paths ...string) *ConfigLoader {
	if len(paths) == 0 {
		paths = []string{
			"config.json",
			"config.yaml",
			"config.yml",
			"bot.json",
			"bot.yaml",
			"bot.yml",
		}
	}
	return &ConfigLoader{
		configPaths: paths,
	}
}

// LoadConfig 加载配置，优先级：环境变量 > 配置文件 > 默认值
func (cl *ConfigLoader) LoadConfig() (*Config, error) {
	config := &Config{}

	// 设置默认值
	if err := setDefaults(config); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	// 从配置文件加载
	if err := cl.loadFromFile(config); err != nil {
		return nil, fmt.Errorf("failed to load from file: %w", err)
	}

	// 从环境变量加载
	if err := loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load from env: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

func (cl *ConfigLoader) loadFromFile(config *Config) error {
	for _, path := range cl.configPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".json":
			if err := json.Unmarshal(data, config); err != nil {
				return fmt.Errorf("failed to parse JSON config %s: %w", path, err)
			}
		case ".yaml", ".yml":
			// 这里需要 yaml 库，暂时只支持 JSON
			return fmt.Errorf("YAML support not implemented, please use JSON config")
		default:
			return fmt.Errorf("unsupported config file format: %s", ext)
		}

		return nil
	}

	// 没找到配置文件，使用默认配置
	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.MaxConcurrentHandlers <= 0 {
		return fmt.Errorf("max_concurrent_handlers must be positive")
	}

	if c.HandlerTimeout <= 0 {
		return fmt.Errorf("handler_timeout must be positive")
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	found := false
	for _, level := range validLogLevels {
		if c.LogLevel == level {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid log_level: %s, must be one of %v", c.LogLevel, validLogLevels)
	}

	return nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) error {
	config.MaxConcurrentHandlers = 100
	config.HandlerTimeout = 30 * time.Second
	config.EnableMetrics = true
	config.LogLevel = "info"

	// 连接默认配置
	config.Connection.ConnectTimeout = 10 * time.Second
	config.Connection.ReadTimeout = 60 * time.Second
	config.Connection.WriteTimeout = 10 * time.Second
	config.Connection.HeartbeatInterval = 30 * time.Second
	config.Connection.HeartbeatTimeout = 5 * time.Second
	config.Connection.AutoReconnect = true
	config.Connection.ReconnectDelay = 5 * time.Second
	config.Connection.MaxReconnectTries = 10

	// HTTP 默认配置
	config.Connection.HTTP.Host = "localhost"
	config.Connection.HTTP.Port = 5700

	// 重试默认配置
	config.Retry = DefaultRetryConfig()

	return nil
}

// loadFromEnv 从环境变量加载配置
func loadFromEnv(config *Config) error {
	if val := os.Getenv("BOT_MAX_CONCURRENT_HANDLERS"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.MaxConcurrentHandlers = intVal
		}
	}

	if val := os.Getenv("BOT_HANDLER_TIMEOUT"); val != "" {
		if dur, err := time.ParseDuration(val); err == nil {
			config.HandlerTimeout = dur
		}
	}

	if val := os.Getenv("BOT_ENABLE_METRICS"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.EnableMetrics = boolVal
		}
	}

	if val := os.Getenv("BOT_LOG_LEVEL"); val != "" {
		config.LogLevel = val
	}

	// WebSocket 配置
	if val := os.Getenv("BOT_WS_URL"); val != "" {
		config.Connection.WebSocket.URL = val
	}

	if val := os.Getenv("BOT_ACCESS_TOKEN"); val != "" {
		config.Connection.WebSocket.AccessToken = val
		config.Connection.HTTP.AccessToken = val
	}

	// HTTP 配置
	if val := os.Getenv("BOT_HTTP_HOST"); val != "" {
		config.Connection.HTTP.Host = val
	}

	if val := os.Getenv("BOT_HTTP_PORT"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.Connection.HTTP.Port = intVal
		}
	}

	if val := os.Getenv("BOT_HTTP_SECRET"); val != "" {
		config.Connection.HTTP.Secret = val
	}

	// 超时配置
	if val := os.Getenv("BOT_CONNECT_TIMEOUT"); val != "" {
		if dur, err := time.ParseDuration(val); err == nil {
			config.Connection.ConnectTimeout = dur
		}
	}

	if val := os.Getenv("BOT_READ_TIMEOUT"); val != "" {
		if dur, err := time.ParseDuration(val); err == nil {
			config.Connection.ReadTimeout = dur
		}
	}

	if val := os.Getenv("BOT_WRITE_TIMEOUT"); val != "" {
		if dur, err := time.ParseDuration(val); err == nil {
			config.Connection.WriteTimeout = dur
		}
	}

	// 重连配置
	if val := os.Getenv("BOT_AUTO_RECONNECT"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.Connection.AutoReconnect = boolVal
		}
	}

	if val := os.Getenv("BOT_RECONNECT_DELAY"); val != "" {
		if dur, err := time.ParseDuration(val); err == nil {
			config.Connection.ReconnectDelay = dur
		}
	}

	return nil
}
