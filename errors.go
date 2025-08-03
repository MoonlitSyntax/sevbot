package bot

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"
)

// BotError 定义机器人相关的错误类型
type BotError struct {
	Type    ErrorType `json:"type"`
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Cause   error     `json:"cause,omitempty"`
}

type ErrorType string

const (
	ErrorTypeConnection ErrorType = "connection"
	ErrorTypeAdapter    ErrorType = "adapter"
	ErrorTypeAPI        ErrorType = "api"
	ErrorTypeEvent      ErrorType = "event"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeRateLimit  ErrorType = "rate_limit"
	ErrorTypeAuth       ErrorType = "auth"
	ErrorTypeUnknown    ErrorType = "unknown"
)

func (e *BotError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%d] %s: %v", e.Type, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%d] %s", e.Type, e.Code, e.Message)
}

func (e *BotError) Unwrap() error {
	return e.Cause
}

func NewBotError(errType ErrorType, code int, message string, cause error) *BotError {
	return &BotError{
		Type:    errType,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts   int           `json:"max_attempts"`
	BaseDelay     time.Duration `json:"base_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	Multiplier    float64       `json:"multiplier"`
	Jitter        bool          `json:"jitter"`
	RetryableFunc func(error) bool
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
		RetryableFunc: func(err error) bool {
			if botErr, ok := err.(*BotError); ok {
				switch botErr.Type {
				case ErrorTypeConnection, ErrorTypeTimeout, ErrorTypeRateLimit:
					return true
				case ErrorTypeAuth, ErrorTypeAPI:
					return false
				default:
					return true
				}
			}
			return true
		},
	}
}

// Retryer 重试器
type Retryer struct {
	config RetryConfig
	logger *slog.Logger
}

func NewRetryer(config RetryConfig) *Retryer {
	return &Retryer{
		config: config,
		logger: slog.Default(),
	}
}

// Do 执行带重试的操作
func (r *Retryer) Do(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			if attempt > 1 {
				r.logger.Info("Operation succeeded after retry",
					"attempt", attempt,
					"total_attempts", r.config.MaxAttempts)
			}
			return nil
		}

		lastErr = err

		if attempt == r.config.MaxAttempts {
			break
		}

		if !r.config.RetryableFunc(err) {
			r.logger.Debug("Error is not retryable, giving up",
				"error", err,
				"attempt", attempt)
			return err
		}

		delay := r.calculateDelay(attempt)
		r.logger.Warn("Operation failed, retrying",
			"error", err,
			"attempt", attempt,
			"next_delay", delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			continue
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w",
		r.config.MaxAttempts, lastErr)
}

func (r *Retryer) calculateDelay(attempt int) time.Duration {
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))

	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	if r.config.Jitter {
		jitter := 0.1 * delay * (2*rand.Float64() - 1)
		delay += jitter
	}

	if delay < 0 {
		delay = float64(r.config.BaseDelay)
	}

	return time.Duration(delay)
}

// ErrorHandler 简化的错误处理器接口
type ErrorHandler interface {
	HandleError(requestID string, event Event, err error) error
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct {
	logger *slog.Logger
}

func NewDefaultErrorHandler() *DefaultErrorHandler {
	return &DefaultErrorHandler{
		logger: slog.Default(),
	}
}

func (h *DefaultErrorHandler) HandleError(requestID string, event Event, err error) error {
	if err == nil {
		return nil
	}

	botErr, ok := err.(*BotError)
	if !ok {
		botErr = NewBotError(ErrorTypeUnknown, 0, err.Error(), err)
	}

	h.logger.Error("Event handler error",
		"request_id", requestID,
		"event_type", fmt.Sprintf("%T", event),
		"error_type", botErr.Type,
		"error_code", botErr.Code,
		"error", botErr.Error())

	// 根据错误类型决定是否继续处理其他处理器
	switch botErr.Type {
	case ErrorTypeAuth:
		// 认证错误，停止处理
		return botErr
	case ErrorTypeRateLimit:
		// 速率限制，可以继续但记录警告
		h.logger.Warn("Rate limit hit", "request_id", requestID)
		return nil
	default:
		// 其他错误，继续处理
		return nil
	}
}