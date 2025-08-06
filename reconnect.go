package sevbot

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// ReconnectStrategy 重连策略接口
type ReconnectStrategy interface {
	ShouldReconnect(attempt int, lastError error) (bool, time.Duration)
	Reset()
}

// ExponentialBackoffStrategy 指数退避重连策略
type ExponentialBackoffStrategy struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	MaxAttempts  int
}

// ShouldReconnect 实现重连策略
func (s *ExponentialBackoffStrategy) ShouldReconnect(attempt int, lastError error) (bool, time.Duration) {
	if s.MaxAttempts > 0 && attempt >= s.MaxAttempts {
		return false, 0
	}

	delay := s.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * s.Multiplier)
		if delay > s.MaxDelay {
			delay = s.MaxDelay
			break
		}
	}

	return true, delay
}

// Reset 重置重连状态
func (s *ExponentialBackoffStrategy) Reset() {
	// 对于指数退避策略，重置是通过attempt计数器外部管理的
}

// ReconnectManager 重连管理器
type ReconnectManager struct {
	strategy       ReconnectStrategy
	connectFunc    func() error
	isConnected    func() bool
	ctx            context.Context
	logger         *slog.Logger
	reconnectCount *atomic.Int64
	enabled        bool
}

// NewReconnectManager 创建重连管理器
func NewReconnectManager(
	strategy ReconnectStrategy,
	connectFunc func() error,
	isConnected func() bool,
	logger *slog.Logger,
) *ReconnectManager {
	return &ReconnectManager{
		strategy:       strategy,
		connectFunc:    connectFunc,
		isConnected:    isConnected,
		logger:         logger,
		reconnectCount: &atomic.Int64{},
		enabled:        true,
	}
}

// SetContext 设置上下文
func (r *ReconnectManager) SetContext(ctx context.Context) {
	r.ctx = ctx
}

// Start 启动重连管理器
func (r *ReconnectManager) Start() {
	if !r.enabled {
		return
	}
	go r.reconnectLoop()
}

// SetEnabled 启用或禁用重连
func (r *ReconnectManager) SetEnabled(enabled bool) {
	r.enabled = enabled
}

// GetReconnectCount 获取重连次数
func (r *ReconnectManager) GetReconnectCount() int64 {
	return r.reconnectCount.Load()
}

// reconnectLoop 重连循环
func (r *ReconnectManager) reconnectLoop() {
	attempt := 0

	for {
		// 等待连接断开
		for r.enabled && r.isConnected() {
			select {
			case <-r.ctx.Done():
				return
			case <-time.After(1 * time.Second):
				// 继续检查连接状态
			}
		}

		if !r.enabled {
			select {
			case <-r.ctx.Done():
				return
			case <-time.After(1 * time.Second):
				continue
			}
		}

		// 连接已断开，开始重连逻辑
		shouldReconnect, delay := r.strategy.ShouldReconnect(attempt, nil)
		if !shouldReconnect {
			r.logger.Error("Maximum reconnection attempts reached, giving up")
			return
		}

		r.logger.Info("Attempting to reconnect",
			"attempt", attempt+1,
			"delay", delay)

		// 等待重连延迟
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(delay):
		}

		// 尝试重连
		if err := r.connectFunc(); err != nil {
			r.logger.Error("Reconnection failed",
				"attempt", attempt+1,
				"error", err)
			attempt++
		} else {
			r.logger.Info("Reconnected successfully",
				"total_attempts", attempt+1)
			r.reconnectCount.Add(1)
			r.strategy.Reset()
			attempt = 0
		}
	}
}

// DefaultReconnectStrategy 返回默认重连策略
func DefaultReconnectStrategy() ReconnectStrategy {
	return &ExponentialBackoffStrategy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  10, // 0表示无限重连
	}
}