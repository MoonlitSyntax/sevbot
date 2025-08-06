package sevbot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"
)

// TraceContext 跟踪上下文键
type TraceContext string

const (
	TraceIDKey     TraceContext = "trace_id"
	SpanIDKey      TraceContext = "span_id"
	RequestIDKey   TraceContext = "request_id"
	ComponentKey   TraceContext = "component"
	OperationKey   TraceContext = "operation"
	StartTimeKey   TraceContext = "start_time"
)

// TraceID 生成新的跟踪ID
func TraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// 回退到时间戳基础的ID
		return generateTimeBasedID()
	}
	return hex.EncodeToString(bytes)
}

// SpanID 生成新的跨度ID
func SpanID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return generateTimeBasedID()[:16]
	}
	return hex.EncodeToString(bytes)
}

// generateTimeBasedID 生成基于时间戳的ID
func generateTimeBasedID() string {
	return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000")))
}

// WithTraceID 向上下文中添加跟踪ID
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// WithSpanID 向上下文中添加跨度ID
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, SpanIDKey, spanID)
}

// WithRequestID 向上下文中添加请求ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithComponent 向上下文中添加组件名称
func WithComponent(ctx context.Context, component string) context.Context {
	return context.WithValue(ctx, ComponentKey, component)
}

// WithOperation 向上下文中添加操作名称
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, OperationKey, operation)
}

// WithStartTime 向上下文中添加开始时间
func WithStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, StartTimeKey, startTime)
}

// GetTraceID 从上下文中获取跟踪ID
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// GetSpanID 从上下文中获取跨度ID
func GetSpanID(ctx context.Context) string {
	if spanID, ok := ctx.Value(SpanIDKey).(string); ok {
		return spanID
	}
	return ""
}

// GetRequestID 从上下文中获取请求ID
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GetComponent 从上下文中获取组件名称
func GetComponent(ctx context.Context) string {
	if component, ok := ctx.Value(ComponentKey).(string); ok {
		return component
	}
	return ""
}

// GetOperation 从上下文中获取操作名称
func GetOperation(ctx context.Context) string {
	if operation, ok := ctx.Value(OperationKey).(string); ok {
		return operation
	}
	return ""
}

// GetStartTime 从上下文中获取开始时间
func GetStartTime(ctx context.Context) time.Time {
	if startTime, ok := ctx.Value(StartTimeKey).(time.Time); ok {
		return startTime
	}
	return time.Time{}
}

// CreateTraceContext 创建一个包含完整跟踪信息的上下文
func CreateTraceContext(ctx context.Context, component, operation string) context.Context {
	traceID := GetTraceID(ctx)
	if traceID == "" {
		traceID = TraceID()
	}
	
	spanID := SpanID()
	startTime := time.Now()
	
	ctx = WithTraceID(ctx, traceID)
	ctx = WithSpanID(ctx, spanID)
	ctx = WithComponent(ctx, component)
	ctx = WithOperation(ctx, operation)
	ctx = WithStartTime(ctx, startTime)
	
	return ctx
}

// LoggerFromContext 从上下文创建带有跟踪信息的日志器
func LoggerFromContext(ctx context.Context, baseLogger *slog.Logger) *slog.Logger {
	logger := baseLogger
	
	if traceID := GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	
	if spanID := GetSpanID(ctx); spanID != "" {
		logger = logger.With("span_id", spanID)
	}
	
	if requestID := GetRequestID(ctx); requestID != "" {
		logger = logger.With("request_id", requestID)
	}
	
	if component := GetComponent(ctx); component != "" {
		logger = logger.With("component", component)
	}
	
	if operation := GetOperation(ctx); operation != "" {
		logger = logger.With("operation", operation)
	}
	
	return logger
}

// LogWithDuration 记录带有持续时间的日志
func LogWithDuration(ctx context.Context, logger *slog.Logger, level slog.Level, msg string, args ...any) {
	contextLogger := LoggerFromContext(ctx, logger)
	
	if startTime := GetStartTime(ctx); !startTime.IsZero() {
		duration := time.Since(startTime)
		args = append(args, "duration", duration)
	}
	
	contextLogger.Log(ctx, level, msg, args...)
}

// StartSpan 开始一个新的跨度
func StartSpan(ctx context.Context, component, operation string) (context.Context, func()) {
	spanCtx := CreateTraceContext(ctx, component, operation)
	
	return spanCtx, func() {
		// 结束跨度时可以执行清理工作
		// 这里可以集成OpenTelemetry或其他追踪系统
	}
}

// TracingMiddleware 创建跟踪中间件
func TracingMiddleware() Middleware {
	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(ctx context.Context, event Event) error {
			// 创建事件处理的跟踪上下文
			traceCtx := CreateTraceContext(ctx, "event_handler", "handle_event")
			
			// 添加事件类型信息
			eventType := GetEventType(event)
			traceCtx = WithOperation(traceCtx, "handle_"+eventType)
			
			// 执行处理器
			return next(traceCtx, event)
		}
	}
}

// GetEventType 获取事件类型字符串
func GetEventType(event Event) string {
	switch event.(type) {
	case *PrivateMessage:
		return "private_message"
	case *GroupMessage:
		return "group_message"
	case *HeartbeatMetaEvent:
		return "heartbeat"
	case *LifecycleMetaEvent:
		return "lifecycle"
	case *FriendRequest:
		return "friend_request"
	case *GroupRequest:
		return "group_request"
	default:
		return "unknown_event"
	}
}

// LoggingMiddleware 增强版日志中间件（使用跟踪上下文）
func TracingLoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(ctx context.Context, event Event) error {
			contextLogger := LoggerFromContext(ctx, logger)
			
			// 记录开始处理
			contextLogger.Info("Starting event processing",
				"event_type", GetEventType(event))
			
			// 执行处理器
			err := next(ctx, event)
			
			// 记录完成状态
			if err != nil {
				LogWithDuration(ctx, contextLogger, slog.LevelError, 
					"Event processing failed", "error", err)
			} else {
				LogWithDuration(ctx, contextLogger, slog.LevelInfo, 
					"Event processing completed")
			}
			
			return err
		}
	}
}