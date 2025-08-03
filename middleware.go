package bot

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

// LoggingMiddleware 日志记录中间件
func LoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(event Event) error {
			start := time.Now()

			// 获取事件类型信息
			eventType := fmt.Sprintf("%T", event)

			// 获取事件基础信息
			var eventInfo map[string]interface{}
			switch e := event.(type) {
			case *PrivateMessage:
				eventInfo = map[string]interface{}{
					"user_id": e.UserID,
					"message": e.RawMessage,
				}
			case *GroupMessage:
				eventInfo = map[string]interface{}{
					"group_id": e.GroupID,
					"user_id":  e.UserID,
					"message":  e.RawMessage,
				}
			case *HeartbeatMetaEvent:
				eventInfo = map[string]interface{}{
					"interval": e.Interval,
					"status":   e.Status,
				}
			default:
				eventInfo = map[string]interface{}{
					"type": eventType,
				}
			}

			logger.Info("Processing event",
				"event_type", eventType,
				"event_info", eventInfo)

			// 执行处理器
			err := next(event)

			duration := time.Since(start)

			if err != nil {
				logger.Error("Event processing failed",
					"event_type", eventType,
					"duration", duration,
					"error", err)
			} else {
				logger.Info("Event processing completed",
					"event_type", eventType,
					"duration", duration)
			}

			return err
		}
	}
}

// RecoveryMiddleware panic恢复中间件
func RecoveryMiddleware(logger *slog.Logger) Middleware {
	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(event Event) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					eventType := fmt.Sprintf("%T", event)

					logger.Error("Panic recovered in event handler",
						"event_type", eventType,
						"panic", r,
						"stack", string(stack))

					err = fmt.Errorf("panic recovered: %v", r)
				}
			}()

			return next(event)
		}
	}
}

// MetricsMiddleware 指标统计中间件
func MetricsMiddleware() Middleware {
	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(event Event) error {
			start := time.Now()

			err := next(event)

			duration := time.Since(start)
			eventType := fmt.Sprintf("%T", event)

			// TODO: 记录指标
			_ = duration
			_ = eventType

			return err
		}
	}
}

// RateLimitMiddleware 简单的速率限制中间件
func RateLimitMiddleware(maxEventsPerSecond int) Middleware {
	ticker := time.NewTicker(time.Second / time.Duration(maxEventsPerSecond))

	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(event Event) error {
			<-ticker.C // 等待令牌
			return next(event)
		}
	}
}

// AuthMiddleware 权限验证中间件
// TODO: 完善
func AuthMiddleware(allowedUsers map[int64]bool) Middleware {
	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(event Event) error {
			var userID int64

			switch e := event.(type) {
			case *PrivateMessage:
				userID = e.UserID
			case *GroupMessage:
				userID = e.UserID
			default:
				// 非用户消息，直接通过
				return next(event)
			}

			if allowedUsers != nil && !allowedUsers[userID] {
				return fmt.Errorf("user %d not authorized", userID)
			}

			return next(event)
		}
	}
}

// FilterMiddleware 事件过滤中间件
func FilterMiddleware(filter func(Event) bool) Middleware {
	return func(next EventHandlerFunc) EventHandlerFunc {
		return func(event Event) error {
			if !filter(event) {
				return nil // 过滤掉，不执行后续处理
			}
			return next(event)
		}
	}
}

func OnlyPrivateMessages(event Event) bool {
	_, ok := event.(*PrivateMessage)
	return ok
}

func OnlyGroupMessages(event Event) bool {
	_, ok := event.(*GroupMessage)
	return ok
}

func OnlyFromUser(userID int64) func(Event) bool {
	return func(event Event) bool {
		switch e := event.(type) {
		case *PrivateMessage:
			return e.UserID == userID
		case *GroupMessage:
			return e.UserID == userID
		}
		return false
	}
}

func OnlyFromGroup(groupID int64) func(Event) bool {
	return func(event Event) bool {
		if e, ok := event.(*GroupMessage); ok {
			return e.GroupID == groupID
		}
		return false
	}
}
