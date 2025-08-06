package sevbot

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

// ValidatedWebSocketConfig 带验证标签的WebSocket配置
type ValidatedWebSocketConfig struct {
	URL         string            `validate:"required,url" json:"url"`
	AccessToken string            `validate:"omitempty" json:"access_token"`
	Headers     map[string]string `validate:"omitempty" json:"headers"`

	// 超时配置
	ConnectTimeout  time.Duration `validate:"min=1s,max=60s" json:"connect_timeout"`
	ReadTimeout     time.Duration `validate:"min=5s,max=300s" json:"read_timeout"`
	WriteTimeout    time.Duration `validate:"min=1s,max=60s" json:"write_timeout"`
	ResponseTimeout time.Duration `validate:"min=5s,max=120s" json:"response_timeout"`

	// 重连配置
	AutoReconnect  bool          `validate:"omitempty" json:"auto_reconnect"`
	ReconnectDelay time.Duration `validate:"min=1s,max=300s" json:"reconnect_delay"`

	// 心跳配置
	PingInterval time.Duration `validate:"min=10s,max=300s" json:"ping_interval"`
}

// ValidatedClientOptions 带验证标签的客户端选项
type ValidatedClientOptions struct {
	// 性能配置
	MaxConcurrentHandlers int           `validate:"min=1,max=10000" json:"max_concurrent_handlers"`
	HandlerTimeout        time.Duration `validate:"min=1s,max=600s" json:"handler_timeout"`

	// 重试配置
	RetryAttempts int `validate:"min=0,max=10" json:"retry_attempts"`

	// 调试选项
	EnableMetrics bool `validate:"omitempty" json:"enable_metrics"`
}

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// ValidateWebSocketConfig 验证WebSocket配置
func ValidateWebSocketConfig(config *WebSocketConfig) error {
	validated := &ValidatedWebSocketConfig{
		URL:             config.URL,
		AccessToken:     config.AccessToken,
		Headers:         config.Headers,
		ConnectTimeout:  config.ConnectTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		ResponseTimeout: config.ResponseTimeout,
		AutoReconnect:   config.AutoReconnect,
		ReconnectDelay:  config.ReconnectDelay,
		PingInterval:    config.PingInterval,
	}

	if err := validate.Struct(validated); err != nil {
		return fmt.Errorf("WebSocket config validation failed: %w", err)
	}
	return nil
}

// ValidateClientOptions 验证客户端选项
func ValidateClientOptions(options *ClientOptions) error {
	validated := &ValidatedClientOptions{
		MaxConcurrentHandlers: options.MaxConcurrentHandlers,
		HandlerTimeout:        options.HandlerTimeout,
		RetryAttempts:         options.RetryAttempts,
		EnableMetrics:         options.EnableMetrics,
	}

	if err := validate.Struct(validated); err != nil {
		return fmt.Errorf("client options validation failed: %w", err)
	}
	return nil
}