package sevbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"time"
)

type Client struct {
	adapter      Adapter
	bus          *EventBus
	logger       *slog.Logger
	ctx          context.Context
	cancelFunc   context.CancelFunc
	semaphore    chan struct{}
	handlerWG    sync.WaitGroup // 用于等待所有处理器完成
	retryer      *Retryer
	errorHandler ErrorHandler
	api          *APIClient
}

// EventHandler 统一的事件处理器函数签名
type EventHandler[T Event] func(ctx context.Context, api *APIClient, event T) error

// Option 配置选项 - 重构后更清晰
type Option func(*ClientOptions)

// ClientOptions 用户配置选项 - 只包含用户真正需要的配置
type ClientOptions struct {
	// 性能配置
	MaxConcurrentHandlers int
	HandlerTimeout        time.Duration

	// 日志配置
	LogLevel string
	Logger   *slog.Logger

	// 重试配置
	RetryAttempts int

	// 调试选项
	EnableMetrics bool
}

// WithMaxConcurrentHandlers 设置最大并发处理数
func WithMaxConcurrentHandlers(max int) Option {
	return func(opts *ClientOptions) {
		opts.MaxConcurrentHandlers = max
	}
}

// WithTimeout 设置处理超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(opts *ClientOptions) {
		opts.HandlerTimeout = timeout
	}
}

// WithLogLevel 设置日志级别
func WithLogLevel(level string) Option {
	return func(opts *ClientOptions) {
		opts.LogLevel = level
	}
}

// WithLogger 设置自定义日志器
func WithLogger(logger *slog.Logger) Option {
	return func(opts *ClientOptions) {
		opts.Logger = logger
	}
}

// WithRetryAttempts 设置重试次数
func WithRetryAttempts(attempts int) Option {
	return func(opts *ClientOptions) {
		opts.RetryAttempts = attempts
	}
}

// WithMetrics 启用或禁用指标收集
func WithMetrics(enable bool) Option {
	return func(opts *ClientOptions) {
		opts.EnableMetrics = enable
	}
}

// New 创建新的客户端实例 - 最简化的接口，满足 80% 的使用场景
func New(wsURL, accessToken string, options ...Option) (*Client, error) {
	// 设置默认选项
	opts := &ClientOptions{
		MaxConcurrentHandlers: 100,
		HandlerTimeout:        30 * time.Second,
		LogLevel:              "info",
		RetryAttempts:         3,
		EnableMetrics:         true,
	}

	// 应用用户选项
	for _, opt := range options {
		opt(opts)
	}

	// 验证客户端选项
	if err := ValidateClientOptions(opts); err != nil {
		return nil, fmt.Errorf("invalid client options: %w", err)
	}

	// 设置默认日志器
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	// 创建 WebSocket 配置 - 用户无需关心这些细节
	wsConfig := WebSocketConfig{
		URL:             wsURL,
		AccessToken:     accessToken,
		Headers:         make(map[string]string),
		ConnectTimeout:  10 * time.Second,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    10 * time.Second,
		ResponseTimeout: 30 * time.Second,
		AutoReconnect:   true,
		ReconnectDelay:  5 * time.Second,
		PingInterval:    30 * time.Second,
	}

	// 验证WebSocket配置
	if err := ValidateWebSocketConfig(&wsConfig); err != nil {
		return nil, fmt.Errorf("invalid WebSocket configuration: %w", err)
	}

	// 创建适配器
	adapter := NewWebSocketAdapter(wsConfig)

	// 创建客户端
	client := &Client{
		adapter:      adapter,
		bus:          NewEventBus(),
		logger:       opts.Logger,
		semaphore:    make(chan struct{}, opts.MaxConcurrentHandlers),
		retryer:      NewRetryer(createRetryConfig(opts.RetryAttempts)),
		errorHandler: NewDefaultErrorHandler(),
	}

	// 创建共享的API客户端
	client.api = NewAPIClient(adapter)

	return client, nil
}

// NewFromConfig 从配置文件创建客户端
func NewFromConfig(configPath string, options ...Option) (*Client, error) {
	// 简化的配置结构
	type SimpleConfig struct {
		URL         string `json:"url"`
		AccessToken string `json:"access_token"`
		LogLevel    string `json:"log_level,omitempty"`
	}

	config, err := loadSimpleConfig[SimpleConfig](configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if config.URL == "" {
		return nil, fmt.Errorf("url is required in config")
	}
	if config.AccessToken == "" {
		return nil, fmt.Errorf("access_token is required in config")
	}

	// 从配置文件设置日志级别
	if config.LogLevel != "" {
		options = append(options, WithLogLevel(config.LogLevel))
	}

	return New(config.URL, config.AccessToken, options...)
}

// createRetryConfig 创建重试配置的辅助函数
func createRetryConfig(maxAttempts int) RetryConfig {
	config := DefaultRetryConfig()
	config.MaxAttempts = maxAttempts
	return config
}

// loadSimpleConfig 通用的简化配置加载器
func loadSimpleConfig[T any](configPath string) (*T, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config T
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	return &config, nil
}

// On 注册事件处理器 - 统一的泛型接口
func On[T Event](c *Client, handler EventHandler[T]) {
	var zero T
	eventType := reflect.TypeOf(zero)

	wrappedHandler := func(ctx context.Context, event Event) error {
		if typedEvent, ok := event.(T); ok {
			// 现在使用传递进来的 context，支持超时和取消
			return handler(ctx, c.api, typedEvent)
		}
		return fmt.Errorf("type assertion failed: expected %T, got %T", zero, event)
	}

	c.bus.Subscribe(eventType, wrappedHandler)
}

// OnPrivateMessage 便捷方法：处理私聊消息
func (c *Client) OnPrivateMessage(handler func(ctx context.Context, api *APIClient, event *PrivateMessage) error) {
	On(c, handler)
}

// OnGroupMessage 便捷方法：处理群消息
func (c *Client) OnGroupMessage(handler func(ctx context.Context, api *APIClient, event *GroupMessage) error) {
	On(c, handler)
}

// OnMessage 便捷方法：处理所有消息类型
func (c *Client) OnMessage(handler func(ctx context.Context, api *APIClient, event Event) error) {
	c.OnPrivateMessage(func(ctx context.Context, api *APIClient, event *PrivateMessage) error {
		return handler(ctx, api, event)
	})
	c.OnGroupMessage(func(ctx context.Context, api *APIClient, event *GroupMessage) error {
		return handler(ctx, api, event)
	})
}

func (c *Client) Use(middleware Middleware) {
	c.bus.Use(middleware)
}

// UseDefaults 使用默认的中间件组合（包含跟踪）
func (c *Client) UseDefaults() {
	c.Use(TracingMiddleware())
	c.Use(RecoveryMiddleware(c.logger))
	c.Use(TracingLoggingMiddleware(c.logger))
}

// UseDefaultsWithoutTracing 使用默认的中间件组合（不包含跟踪）
func (c *Client) UseDefaultsWithoutTracing() {
	c.Use(RecoveryMiddleware(c.logger))
	c.Use(LoggingMiddleware(c.logger))
}

// UseRecovery 添加panic恢复中间件
func (c *Client) UseRecovery() {
	c.Use(RecoveryMiddleware(c.logger))
}

// UseLogging 添加日志中间件
func (c *Client) UseLogging() {
	c.Use(LoggingMiddleware(c.logger))
}

// UseTracing 添加跟踪中间件
func (c *Client) UseTracing() {
	c.Use(TracingMiddleware())
	c.Use(TracingLoggingMiddleware(c.logger))
}

// UseMetrics 添加指标中间件
func (c *Client) UseMetrics() {
	c.Use(MetricsMiddleware())
}

// UseRateLimit 添加速率限制中间件
func (c *Client) UseRateLimit(maxEventsPerSecond int) {
	c.Use(RateLimitMiddleware(maxEventsPerSecond))
}

// UseFilter 添加事件过滤中间件
func (c *Client) UseFilter(filter func(Event) bool) {
	c.Use(FilterMiddleware(filter))
}

func (c *Client) SetErrorHandler(handler ErrorHandler) {
	c.errorHandler = handler
}

func (c *Client) SetRetryer(retryer *Retryer) {
	c.retryer = retryer
}

// API 获取API客户端
func (c *Client) API() *APIClient {
	return c.api
}

// Reply 便捷回复方法 - 现在需要显式传入 context
func (c *Client) Reply(ctx context.Context, event interface{}, message MessageChain) (*SendMessageResponse, error) {
	switch e := event.(type) {
	case *PrivateMessage:
		return c.api.SendPrivateMessage(ctx, e.UserID, message)
	case *GroupMessage:
		return c.api.SendGroupMessage(ctx, e.GroupID, message)
	default:
		return nil, fmt.Errorf("unsupported event type for reply: %T", event)
	}
}

// ReplyText 便捷文本回复方法
func (c *Client) ReplyText(ctx context.Context, event interface{}, text string) (*SendMessageResponse, error) {
	return c.Reply(ctx, event, TextMessage(text))
}

// SendPrivateMessage 发送私聊消息
func (c *Client) SendPrivateMessage(ctx context.Context, userID int64, message MessageChain) (*SendMessageResponse, error) {
	return c.api.SendPrivateMessage(ctx, userID, message)
}

// SendGroupMessage 发送群消息
func (c *Client) SendGroupMessage(ctx context.Context, groupID int64, message MessageChain) (*SendMessageResponse, error) {
	return c.api.SendGroupMessage(ctx, groupID, message)
}

func (c *Client) Run() error {
	if c.adapter == nil {
		return fmt.Errorf("adapter is not set")
	}

	// 创建带有跟踪ID的根上下文
	rootCtx := WithTraceID(context.Background(), TraceID())
	c.ctx, c.cancelFunc = context.WithCancel(rootCtx)

	// 使用跟踪上下文创建日志器
	traceLogger := LoggerFromContext(c.ctx, c.logger)

	if err := c.adapter.Connect(c.ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	traceLogger.Info("Client is running with tracing enabled",
		"trace_id", GetTraceID(c.ctx))
	go c.eventLoop()

	<-c.ctx.Done()
	return nil
}

func (c *Client) Stop() error {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}

	// 使用 WaitGroup 等待所有处理器完成，避免死锁风险
	done := make(chan struct{})
	go func() {
		c.handlerWG.Wait()
		close(done)
	}()

	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	select {
	case <-done:
		c.logger.Info("All handlers completed gracefully")
	case <-timeout.C:
		c.logger.Warn("Timeout waiting for handlers to complete")
	}

	return c.adapter.Disconnect()
}

func (c *Client) eventLoop() {
	defer func() {
		if err := c.adapter.Disconnect(); err != nil {
			c.logger.Error("Failed to disconnect adapter", "error", err)
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Client is stopping...")
			return
		case rawEvent, ok := <-c.adapter.EventChannel():
			if !ok {
				c.logger.Info("Event channel closed, stopping client...")
				return
			}
			if err := c.bus.dispatch(c, rawEvent); err != nil {
				c.logger.Error("Event dispatch failed",
					"error", err,
					"rawEvent", string(rawEvent))
			}
		}
	}
}
