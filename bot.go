package bot

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"
)

type Bot struct {
	adapter      Adapter
	bus          *EventBus
	logger       *slog.Logger
	config       *Config
	ctx          context.Context
	cancelFunc   context.CancelFunc
	semaphore    chan struct{}
	retryer      *Retryer
	errorHandler ErrorHandler
	api          *APIClient
}

// EventHandler 简化的事件处理器函数签名
type EventHandler[T Event] func(api *APIClient, event T) error

func NewBot(adapter Adapter, config *Config) *Bot {
	if config == nil {
		config = &Config{
			MaxConcurrentHandlers: 100,
			HandlerTimeout:        30 * time.Second,
			EnableMetrics:         true,
			LogLevel:              "info",
		}
	}

	bot := &Bot{
		adapter:      adapter,
		bus:          NewEventBus(),
		logger:       slog.Default(),
		config:       config,
		semaphore:    make(chan struct{}, config.MaxConcurrentHandlers),
		retryer:      NewRetryer(DefaultRetryConfig()),
		errorHandler: NewDefaultErrorHandler(),
	}

	// 创建共享的API客户端
	bot.api = NewAPIClient(adapter)

	return bot
}

// NewWebSocketBot 创建WebSocket机器人实例
func NewWebSocketBot(config WebSocketConfig) *Bot {
	adapter := NewWebSocketAdapter(config)
	return NewBot(adapter, nil)
}

// NewWebSocketBotSimple 创建WebSocket机器人实例
func NewWebSocketBotSimple(url, token string) *Bot {
	config := DefaultWebSocketConfig(url)
	config.AccessToken = token
	return NewWebSocketBot(config)
}

// On 注册事件处理器 - 泛型版本，更简洁
func On[T Event](b *Bot, handler EventHandler[T]) {
	var zero T
	eventType := reflect.TypeOf(zero)

	wrappedHandler := func(event Event) error {
		if typedEvent, ok := event.(T); ok {
			return handler(b.api, typedEvent)
		}
		return fmt.Errorf("type assertion failed: expected %T, got %T", zero, event)
	}

	b.bus.Subscribe(eventType, wrappedHandler)
}

// OnMessage 便捷方法：处理所有消息
func (b *Bot) OnMessage(handler func(api *APIClient, event Event) error) {
	On(b, func(api *APIClient, event *PrivateMessage) error {
		return handler(api, event)
	})
	On(b, func(api *APIClient, event *GroupMessage) error {
		return handler(api, event)
	})
}

// OnPrivateMessage 便捷方法：处理私聊消息
func (b *Bot) OnPrivateMessage(handler func(api *APIClient, event *PrivateMessage) error) {
	On(b, handler)
}

// OnGroupMessage 便捷方法：处理群消息
func (b *Bot) OnGroupMessage(handler func(api *APIClient, event *GroupMessage) error) {
	On(b, handler)
}

func (b *Bot) Use(middleware Middleware) {
	b.bus.Use(middleware)
}

// UseDefaults 使用默认的中间件组合
func (b *Bot) UseDefaults() {
	b.Use(RecoveryMiddleware(b.logger))
	b.Use(LoggingMiddleware(b.logger))
}

// UseRecovery 添加panic恢复中间件
func (b *Bot) UseRecovery() {
	b.Use(RecoveryMiddleware(b.logger))
}

// UseLogging 添加日志中间件
func (b *Bot) UseLogging() {
	b.Use(LoggingMiddleware(b.logger))
}

// UseMetrics 添加指标中间件
func (b *Bot) UseMetrics() {
	b.Use(MetricsMiddleware())
}

// UseRateLimit 添加速率限制中间件
func (b *Bot) UseRateLimit(maxEventsPerSecond int) {
	b.Use(RateLimitMiddleware(maxEventsPerSecond))
}

// UseFilter 添加事件过滤中间件
func (b *Bot) UseFilter(filter func(Event) bool) {
	b.Use(FilterMiddleware(filter))
}

func (b *Bot) SetErrorHandler(handler ErrorHandler) {
	b.errorHandler = handler
}

func (b *Bot) SetRetryer(retryer *Retryer) {
	b.retryer = retryer
}

// API 获取API客户端
func (b *Bot) API() *APIClient {
	return b.api
}

func (b *Bot) Run() error {
	if b.adapter == nil {
		return fmt.Errorf("adapter is not set")
	}

	b.ctx, b.cancelFunc = context.WithCancel(context.Background())

	if err := b.adapter.Connect(b.ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	b.logger.Info("Bot is running...")
	go b.eventLoop()

	<-b.ctx.Done()
	return nil
}

func (b *Bot) Stop() error {
	if b.cancelFunc != nil {
		b.cancelFunc()
	}

	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	done := make(chan struct{})
	go func() {
		for i := 0; i < cap(b.semaphore); i++ {
			b.semaphore <- struct{}{}
		}
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("All handlers completed")
	case <-timeout.C:
		b.logger.Warn("Timeout waiting for handlers to complete")
	}

	return b.adapter.Disconnect()
}

func (b *Bot) eventLoop() {
	defer func() {
		if err := b.adapter.Disconnect(); err != nil {
			b.logger.Error("Failed to disconnect adapter", "error", err)
		}
	}()

	for {
		select {
		case <-b.ctx.Done():
			b.logger.Info("Bot is stopping...")
			return
		case rawEvent, ok := <-b.adapter.EventChannel():
			if !ok {
				b.logger.Info("Event channel closed, stopping bot...")
				return
			}
			if err := b.bus.dispatch(b, rawEvent); err != nil {
				b.logger.Error("Event dispatch failed",
					"error", err,
					"rawEvent", string(rawEvent))
			}
		}
	}
}
